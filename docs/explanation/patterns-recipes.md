# Patterns & Recipes

This guide covers common patterns and practical recipes for building state machines with Statekit.

## Table of Contents

1. [Retry with Exponential Backoff](#retry-with-exponential-backoff)
2. [Request/Response Pattern](#requestresponse-pattern)
3. [Wizard/Multi-Step Forms](#wizardmulti-step-forms)
4. [Authentication Flow](#authentication-flow)
5. [Rate Limiting](#rate-limiting)
6. [Circuit Breaker](#circuit-breaker)
7. [Saga/Compensation Pattern](#sagacompensation-pattern)
8. [Polling with Backoff](#polling-with-backoff)
9. [Debounce/Throttle](#debouncethrottle)
10. [Optimistic Updates](#optimistic-updates)

---

## Retry with Exponential Backoff

Handle transient failures with automatic retry and increasing delays.

```go
type RetryContext struct {
    Attempt     int
    MaxAttempts int
    LastError   error
    Result      any
}

machine, _ := statekit.NewMachine[RetryContext]("retry").
    WithInitial("idle").
    WithContext(RetryContext{MaxAttempts: 3}).
    WithAction("incrementAttempt", func(ctx *RetryContext, e statekit.Event) {
        ctx.Attempt++
    }).
    WithAction("saveError", func(ctx *RetryContext, e statekit.Event) {
        if err, ok := e.Payload.(error); ok {
            ctx.LastError = err
        }
    }).
    WithAction("saveResult", func(ctx *RetryContext, e statekit.Event) {
        ctx.Result = e.Payload
    }).
    WithAction("resetAttempts", func(ctx *RetryContext, e statekit.Event) {
        ctx.Attempt = 0
        ctx.LastError = nil
    }).
    WithGuard("canRetry", func(ctx RetryContext, e statekit.Event) bool {
        return ctx.Attempt < ctx.MaxAttempts
    }).
    WithGuard("maxRetriesReached", func(ctx RetryContext, e statekit.Event) bool {
        return ctx.Attempt >= ctx.MaxAttempts
    }).
    // States
    State("idle").
        On("START").Target("attempting").Do("resetAttempts").
    Done().
    State("attempting").
        OnEntry("incrementAttempt").
        On("SUCCESS").Target("success").Do("saveResult").
        On("FAILURE").Target("waiting").Guard("canRetry").Do("saveError").
        On("FAILURE").Target("failed").Guard("maxRetriesReached").Do("saveError").
    Done().
    State("waiting").
        // Exponential backoff: 1s, 2s, 4s...
        After(1 * time.Second).Target("attempting").  // Base delay
    Done().
    State("success").Final().Done().
    State("failed").Final().Done().
    Build()
```

**Dynamic backoff** (using context to calculate delay):

```go
// In your service/action, calculate delay based on attempt
func calculateBackoff(attempt int) time.Duration {
    base := time.Second
    return base * time.Duration(1<<uint(attempt-1)) // 1s, 2s, 4s, 8s...
}
```

---

## Request/Response Pattern

Model async request handling with loading, success, and error states.

```go
type RequestContext struct {
    RequestID string
    Data      any
    Error     error
    Loading   bool
}

machine, _ := statekit.NewMachine[RequestContext]("request").
    WithInitial("idle").
    WithAction("setLoading", func(ctx *RequestContext, e statekit.Event) {
        ctx.Loading = true
        ctx.Error = nil
    }).
    WithAction("setData", func(ctx *RequestContext, e statekit.Event) {
        ctx.Data = e.Payload
        ctx.Loading = false
    }).
    WithAction("setError", func(ctx *RequestContext, e statekit.Event) {
        if err, ok := e.Payload.(error); ok {
            ctx.Error = err
        }
        ctx.Loading = false
    }).
    WithAction("clearData", func(ctx *RequestContext, e statekit.Event) {
        ctx.Data = nil
        ctx.Error = nil
    }).
    State("idle").
        On("FETCH").Target("loading").Do("setLoading").
    Done().
    State("loading").
        On("SUCCESS").Target("success").Do("setData").
        On("ERROR").Target("error").Do("setError").
        On("CANCEL").Target("idle").Do("clearData").
    Done().
    State("success").
        On("REFRESH").Target("loading").Do("setLoading").
        On("RESET").Target("idle").Do("clearData").
    Done().
    State("error").
        On("RETRY").Target("loading").Do("setLoading").
        On("RESET").Target("idle").Do("clearData").
    Done().
    Build()
```

---

## Wizard/Multi-Step Forms

Navigate through form steps with validation and history support.

```go
type WizardContext struct {
    CurrentStep int
    TotalSteps  int
    Data        map[string]any
    Errors      map[string]string
}

machine, _ := statekit.NewMachine[WizardContext]("wizard").
    WithInitial("form").
    WithContext(WizardContext{
        TotalSteps: 3,
        Data:       make(map[string]any),
        Errors:     make(map[string]string),
    }).
    WithAction("saveStepData", func(ctx *WizardContext, e statekit.Event) {
        if data, ok := e.Payload.(map[string]any); ok {
            for k, v := range data {
                ctx.Data[k] = v
            }
        }
    }).
    WithAction("clearErrors", func(ctx *WizardContext, e statekit.Event) {
        ctx.Errors = make(map[string]string)
    }).
    WithAction("setErrors", func(ctx *WizardContext, e statekit.Event) {
        if errs, ok := e.Payload.(map[string]string); ok {
            ctx.Errors = errs
        }
    }).
    WithGuard("isValid", func(ctx WizardContext, e statekit.Event) bool {
        return len(ctx.Errors) == 0
    }).
    // Main form compound state with history
    State("form").
        WithInitial("step1").
        On("CANCEL").Target("cancelled").End().
        History("formHistory").Shallow().Default("step1").End().
        // Step 1
        State("step1").
            On("NEXT").Target("step2").Guard("isValid").Do("saveStepData").
            On("VALIDATE_FAIL").Target("step1").Do("setErrors").
        End().End().
        // Step 2
        State("step2").
            On("NEXT").Target("step3").Guard("isValid").Do("saveStepData").
            On("BACK").Target("step1").
            On("VALIDATE_FAIL").Target("step2").Do("setErrors").
        End().End().
        // Step 3
        State("step3").
            On("SUBMIT").Target("submitting").Guard("isValid").Do("saveStepData").
            On("BACK").Target("step2").
            On("VALIDATE_FAIL").Target("step3").Do("setErrors").
        End().End().
    Done().
    State("submitting").
        On("SUCCESS").Target("complete").
        On("ERROR").Target("form").  // Returns to last step via history
    Done().
    State("complete").Final().Done().
    State("cancelled").Final().Done().
    Build()
```

---

## Authentication Flow

Handle login, session management, and token refresh.

```go
type AuthContext struct {
    User         *User
    AccessToken  string
    RefreshToken string
    ExpiresAt    time.Time
    Error        error
}

machine, _ := statekit.NewMachine[AuthContext]("auth").
    WithInitial("unauthenticated").
    WithAction("setTokens", func(ctx *AuthContext, e statekit.Event) {
        if tokens, ok := e.Payload.(TokenResponse); ok {
            ctx.AccessToken = tokens.Access
            ctx.RefreshToken = tokens.Refresh
            ctx.ExpiresAt = time.Now().Add(tokens.ExpiresIn)
        }
    }).
    WithAction("setUser", func(ctx *AuthContext, e statekit.Event) {
        if user, ok := e.Payload.(*User); ok {
            ctx.User = user
        }
    }).
    WithAction("clearAuth", func(ctx *AuthContext, e statekit.Event) {
        ctx.User = nil
        ctx.AccessToken = ""
        ctx.RefreshToken = ""
    }).
    WithAction("setError", func(ctx *AuthContext, e statekit.Event) {
        if err, ok := e.Payload.(error); ok {
            ctx.Error = err
        }
    }).
    WithGuard("hasRefreshToken", func(ctx AuthContext, e statekit.Event) bool {
        return ctx.RefreshToken != ""
    }).
    // States
    State("unauthenticated").
        On("LOGIN").Target("authenticating").
        On("RESTORE_SESSION").Target("refreshing").Guard("hasRefreshToken").
    Done().
    State("authenticating").
        On("SUCCESS").Target("authenticated").Do("setTokens", "setUser").
        On("FAILURE").Target("unauthenticated").Do("setError").
    Done().
    State("authenticated").
        WithInitial("active").
        On("LOGOUT").Target("unauthenticated").Do("clearAuth").End().
        // Active session
        State("active").
            After(50 * time.Minute).Target("refreshing").  // Refresh before expiry
            On("TOKEN_EXPIRED").Target("refreshing").
        End().End().
    Done().
    State("refreshing").
        On("SUCCESS").Target("authenticated").Do("setTokens").
        On("FAILURE").Target("unauthenticated").Do("clearAuth").
    Done().
    Build()
```

---

## Rate Limiting

Implement token bucket rate limiting.

```go
type RateLimitContext struct {
    Tokens       int
    MaxTokens    int
    RefillRate   int           // tokens per interval
    RefillPeriod time.Duration
    LastRefill   time.Time
}

machine, _ := statekit.NewMachine[RateLimitContext]("ratelimit").
    WithInitial("ready").
    WithContext(RateLimitContext{
        Tokens:       10,
        MaxTokens:    10,
        RefillRate:   1,
        RefillPeriod: time.Second,
    }).
    WithAction("consumeToken", func(ctx *RateLimitContext, e statekit.Event) {
        ctx.Tokens--
    }).
    WithAction("refillTokens", func(ctx *RateLimitContext, e statekit.Event) {
        elapsed := time.Since(ctx.LastRefill)
        tokensToAdd := int(elapsed/ctx.RefillPeriod) * ctx.RefillRate
        ctx.Tokens = min(ctx.Tokens+tokensToAdd, ctx.MaxTokens)
        ctx.LastRefill = time.Now()
    }).
    WithGuard("hasTokens", func(ctx RateLimitContext, e statekit.Event) bool {
        return ctx.Tokens > 0
    }).
    WithGuard("noTokens", func(ctx RateLimitContext, e statekit.Event) bool {
        return ctx.Tokens <= 0
    }).
    State("ready").
        On("REQUEST").Target("ready").Guard("hasTokens").Do("consumeToken").
        On("REQUEST").Target("limited").Guard("noTokens").
    Done().
    State("limited").
        After(1 * time.Second).Target("ready").Do("refillTokens").
    Done().
    Build()
```

---

## Circuit Breaker

Protect services with automatic failure detection and recovery.

```go
type CircuitContext struct {
    FailureCount     int
    SuccessCount     int
    FailureThreshold int
    SuccessThreshold int // for half-open
    LastFailure      time.Time
}

machine, _ := statekit.NewMachine[CircuitContext]("circuit").
    WithInitial("closed").
    WithContext(CircuitContext{
        FailureThreshold: 5,
        SuccessThreshold: 3,
    }).
    WithAction("recordFailure", func(ctx *CircuitContext, e statekit.Event) {
        ctx.FailureCount++
        ctx.LastFailure = time.Now()
    }).
    WithAction("recordSuccess", func(ctx *CircuitContext, e statekit.Event) {
        ctx.SuccessCount++
    }).
    WithAction("resetCounters", func(ctx *CircuitContext, e statekit.Event) {
        ctx.FailureCount = 0
        ctx.SuccessCount = 0
    }).
    WithGuard("thresholdReached", func(ctx CircuitContext, e statekit.Event) bool {
        return ctx.FailureCount >= ctx.FailureThreshold
    }).
    WithGuard("recoverySuccessful", func(ctx CircuitContext, e statekit.Event) bool {
        return ctx.SuccessCount >= ctx.SuccessThreshold
    }).
    // Closed: normal operation
    State("closed").
        On("SUCCESS").Target("closed").Do("resetCounters").
        On("FAILURE").Target("closed").Do("recordFailure").
        On("FAILURE").Target("open").Guard("thresholdReached").
    Done().
    // Open: reject all requests
    State("open").
        OnEntry("resetCounters").
        After(30 * time.Second).Target("half_open").  // Recovery timeout
        On("REQUEST").Target("open").  // Reject immediately
    Done().
    // Half-open: test if service recovered
    State("half_open").
        OnEntry("resetCounters").
        On("SUCCESS").Target("half_open").Do("recordSuccess").
        On("SUCCESS").Target("closed").Guard("recoverySuccessful").
        On("FAILURE").Target("open").
    Done().
    Build()
```

---

## Saga/Compensation Pattern

Execute distributed transactions with automatic rollback on failure.

```go
type SagaContext struct {
    OrderID       string
    PaymentID     string
    InventoryID   string
    ShippingID    string
    CompletedSteps []string
    Error         error
}

machine, _ := statekit.NewMachine[SagaContext]("order_saga").
    WithInitial("idle").
    WithAction("markStep", func(ctx *SagaContext, e statekit.Event) {
        if step, ok := e.Payload.(string); ok {
            ctx.CompletedSteps = append(ctx.CompletedSteps, step)
        }
    }).
    WithAction("setError", func(ctx *SagaContext, e statekit.Event) {
        if err, ok := e.Payload.(error); ok {
            ctx.Error = err
        }
    }).
    WithGuard("paymentDone", func(ctx SagaContext, e statekit.Event) bool {
        return contains(ctx.CompletedSteps, "payment")
    }).
    WithGuard("inventoryDone", func(ctx SagaContext, e statekit.Event) bool {
        return contains(ctx.CompletedSteps, "inventory")
    }).
    // Forward path
    State("idle").
        On("START").Target("reserving_payment").
    Done().
    State("reserving_payment").
        On("SUCCESS").Target("reserving_inventory").Do("markStep").
        On("FAILURE").Target("failed").Do("setError").
    Done().
    State("reserving_inventory").
        On("SUCCESS").Target("creating_shipment").Do("markStep").
        On("FAILURE").Target("compensating_payment").Do("setError").
    Done().
    State("creating_shipment").
        On("SUCCESS").Target("completed").Do("markStep").
        On("FAILURE").Target("compensating_inventory").Do("setError").
    Done().
    State("completed").Final().Done().
    // Compensation path (reverse order)
    State("compensating_inventory").
        On("COMPENSATED").Target("compensating_payment").
        On("COMPENSATION_FAILED").Target("compensation_failed").
    Done().
    State("compensating_payment").
        On("COMPENSATED").Target("failed").
        On("COMPENSATION_FAILED").Target("compensation_failed").
    Done().
    State("failed").Final().Done().
    State("compensation_failed").Final().Done().
    Build()
```

---

## Polling with Backoff

Poll for status updates with adaptive intervals.

```go
type PollingContext struct {
    PollCount     int
    MaxPolls      int
    LastStatus    string
    PollInterval  time.Duration
    MaxInterval   time.Duration
}

machine, _ := statekit.NewMachine[PollingContext]("polling").
    WithInitial("idle").
    WithContext(PollingContext{
        MaxPolls:     100,
        PollInterval: time.Second,
        MaxInterval:  30 * time.Second,
    }).
    WithAction("incrementPoll", func(ctx *PollingContext, e statekit.Event) {
        ctx.PollCount++
    }).
    WithAction("updateStatus", func(ctx *PollingContext, e statekit.Event) {
        if status, ok := e.Payload.(string); ok {
            ctx.LastStatus = status
        }
    }).
    WithAction("increaseInterval", func(ctx *PollingContext, e statekit.Event) {
        ctx.PollInterval = min(ctx.PollInterval*2, ctx.MaxInterval)
    }).
    WithAction("resetInterval", func(ctx *PollingContext, e statekit.Event) {
        ctx.PollInterval = time.Second
    }).
    WithGuard("notComplete", func(ctx PollingContext, e statekit.Event) bool {
        return ctx.LastStatus != "complete" && ctx.PollCount < ctx.MaxPolls
    }).
    WithGuard("isComplete", func(ctx PollingContext, e statekit.Event) bool {
        return ctx.LastStatus == "complete"
    }).
    WithGuard("maxPollsReached", func(ctx PollingContext, e statekit.Event) bool {
        return ctx.PollCount >= ctx.MaxPolls
    }).
    State("idle").
        On("START").Target("polling").
    Done().
    State("polling").
        OnEntry("incrementPoll").
        On("STATUS").Target("waiting").Guard("notComplete").Do("updateStatus", "increaseInterval").
        On("STATUS").Target("complete").Guard("isComplete").Do("updateStatus").
        On("STATUS").Target("timeout").Guard("maxPollsReached").
    Done().
    State("waiting").
        After(1 * time.Second).Target("polling").  // Uses context.PollInterval in practice
    Done().
    State("complete").Final().Done().
    State("timeout").Final().Done().
    Build()
```

---

## Debounce/Throttle

Debounce rapid inputs before processing.

```go
type DebounceContext struct {
    PendingValue any
    LastValue    any
    DebounceMs   int
}

machine, _ := statekit.NewMachine[DebounceContext]("debounce").
    WithInitial("idle").
    WithContext(DebounceContext{DebounceMs: 300}).
    WithAction("storePending", func(ctx *DebounceContext, e statekit.Event) {
        ctx.PendingValue = e.Payload
    }).
    WithAction("commitValue", func(ctx *DebounceContext, e statekit.Event) {
        ctx.LastValue = ctx.PendingValue
        ctx.PendingValue = nil
    }).
    State("idle").
        On("INPUT").Target("debouncing").Do("storePending").
    Done().
    State("debouncing").
        On("INPUT").Target("debouncing").Do("storePending").  // Reset timer
        After(300 * time.Millisecond).Target("idle").Do("commitValue").
    Done().
    Build()
```

**Throttle** (process at most once per interval):

```go
machine, _ := statekit.NewMachine[ThrottleContext]("throttle").
    WithInitial("ready").
    WithAction("processValue", func(ctx *ThrottleContext, e statekit.Event) {
        ctx.LastValue = e.Payload
        ctx.LastProcessed = time.Now()
    }).
    State("ready").
        On("INPUT").Target("cooldown").Do("processValue").
    Done().
    State("cooldown").
        On("INPUT").Target("cooldown").  // Ignore during cooldown
        After(100 * time.Millisecond).Target("ready").
    Done().
    Build()
```

---

## Optimistic Updates

Apply changes immediately, rollback on server failure.

```go
type OptimisticContext struct {
    CurrentValue  any
    OptimisticVal any
    PendingUpdate any
    RollbackVal   any
}

machine, _ := statekit.NewMachine[OptimisticContext]("optimistic").
    WithInitial("idle").
    WithAction("applyOptimistic", func(ctx *OptimisticContext, e statekit.Event) {
        ctx.RollbackVal = ctx.CurrentValue
        ctx.PendingUpdate = e.Payload
        ctx.CurrentValue = e.Payload  // Apply immediately
    }).
    WithAction("confirmUpdate", func(ctx *OptimisticContext, e statekit.Event) {
        ctx.RollbackVal = nil
        ctx.PendingUpdate = nil
    }).
    WithAction("rollback", func(ctx *OptimisticContext, e statekit.Event) {
        ctx.CurrentValue = ctx.RollbackVal
        ctx.RollbackVal = nil
        ctx.PendingUpdate = nil
    }).
    State("idle").
        On("UPDATE").Target("pending").Do("applyOptimistic").
    Done().
    State("pending").
        On("SERVER_SUCCESS").Target("idle").Do("confirmUpdate").
        On("SERVER_FAILURE").Target("idle").Do("rollback").
        After(10 * time.Second).Target("idle").Do("rollback").  // Timeout
    Done().
    Build()
```

---

## Best Practices

### 1. Keep Context Focused

```go
// Good: Focused context for the domain
type OrderContext struct {
    OrderID string
    Status  OrderStatus
    Items   []Item
    Total   Money
}

// Avoid: Kitchen sink context
type BadContext struct {
    Order      Order
    User       User
    Config     Config
    Logger     *log.Logger  // Don't put dependencies here
}
```

### 2. Use Guards for Branching

```go
// Good: Clear guard-based branching
On("SUBMIT").Target("processing").Guard("isValid").
On("SUBMIT").Target("validation_error").Guard("hasErrors").

// Avoid: Complex logic in actions
```

### 3. Prefer Compound States for Related Logic

```go
// Good: Group related states
State("checkout").
    WithInitial("cart").
    On("CANCEL").Target("cancelled").End().  // Applies to all children
    State("cart")./* ... */.End().End().
    State("payment")./* ... */.End().End().
    State("confirmation")./* ... */.End().End().
Done()
```

### 4. Use History for "Resume" Behavior

```go
// When returning from interruptions
State("workflow").
    History("resume").Shallow().Default("step1").End().
    On("INTERRUPT").Target("paused").End().
Done()
State("paused").
    On("RESUME").Target("resume").  // Returns to last step
Done()
```

### 5. Model Timeouts Explicitly

```go
// Good: Explicit timeout handling
State("waiting_for_response").
    On("RESPONSE").Target("processing").
    After(30 * time.Second).Target("timeout").
Done()
State("timeout").
    On("RETRY").Target("requesting").
    On("ABORT").Target("failed").
Done()
```

---

## See Also

- [Getting Started](../tutorials/getting-started.md)
- [Hierarchical States](../how-to/hierarchical-states.md)
- [XState Migration](../tutorials/xstate-migration.md)
- [API Reference](../reference/api-reference.md)
