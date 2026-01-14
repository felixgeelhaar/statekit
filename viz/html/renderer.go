package html

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/felixgeelhaar/statekit/viz"
)

// Renderer generates a standalone HTML visualizer.
type Renderer struct{}

// NewRenderer creates a new HTML renderer.
func NewRenderer() *Renderer {
	return &Renderer{}
}

// Render returns the HTML content for the given machine.
func (r *Renderer) Render(machine *viz.VizMachine) (string, error) {
	// Marshal machine to JSON
	jsonData, err := json.Marshal(machine)
	if err != nil {
		return "", fmt.Errorf("marshal machine: %w", err)
	}

	data := struct {
		MachineJSON template.JS
	}{
		// json.Marshal escapes HTML characters by default, so casting to template.JS is safe here.
		// We need raw JSON for the script tag.
		// #nosec G203
		MachineJSON: template.JS(jsonData),
	}

	var buf bytes.Buffer
	if err := htmlTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// TODO: In a real implementation, we might want to inline these scripts or provide a way to customize them.
var htmlTemplate = template.Must(template.New("viz").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Statekit Visualizer</title>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/cytoscape/3.28.1/cytoscape.min.js"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/dagre/0.8.5/dagre.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/cytoscape-dagre@2.5.0/cytoscape-dagre.min.js"></script>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; margin: 0; padding: 0; display: flex; height: 100vh; overflow: hidden; }
        #sidebar { width: 300px; background: #f8f9fa; border-right: 1px solid #ddd; display: flex; flex-direction: column; z-index: 10; }
        #graph { flex: 1; background: #fff; }
        .header { padding: 1rem; border-bottom: 1px solid #ddd; background: #fff; }
        .header h1 { margin: 0; font-size: 1.2rem; }
        .controls { padding: 1rem; flex: 1; overflow-y: auto; }
        .panel { margin-bottom: 1.5rem; }
        .panel-title { font-weight: 600; margin-bottom: 0.5rem; font-size: 0.9rem; text-transform: uppercase; color: #666; }
        .state-tag { display: inline-block; padding: 0.2rem 0.5rem; background: #e9ecef; border-radius: 4px; font-size: 0.9rem; margin-bottom: 0.5rem; }
        .state-tag.active { background: #228be6; color: white; }
        button { display: block; width: 100%; padding: 0.5rem; margin-bottom: 0.5rem; background: #fff; border: 1px solid #ced4da; border-radius: 4px; cursor: pointer; text-align: left; transition: all 0.2s; }
        button:hover:not(:disabled) { background: #e9ecef; border-color: #adb5bd; }
        button:disabled { opacity: 0.5; cursor: not-allowed; }
        button .event-name { font-weight: 600; }
        button .target-name { font-size: 0.8rem; color: #666; }
        
        /* Graph styles */
        #graph { width: 100%; height: 100%; }
    </style>
</head>
<body>
    <div id="sidebar">
        <div class="header">
            <h1>Statekit Viz</h1>
            <div id="machine-id" style="font-size: 0.9rem; color: #666;"></div>
        </div>
        <div class="controls">
            <div class="panel">
                <div class="panel-title">Current State</div>
                <div id="current-state" class="state-tag active"></div>
            </div>
            
            <div class="panel">
                <div class="panel-title">Available Events</div>
                <div id="events-list"></div>
            </div>

            <div class="panel">
                <div class="panel-title">Context</div>
                <pre id="context-display" style="font-size: 0.8rem; background: #eee; padding: 0.5rem; border-radius: 4px; overflow: auto;">{}</pre>
            </div>
        </div>
    </div>
    <div id="graph"></div>

    <script>
        // Native JSON data injected from Go
        const machine = {{.MachineJSON}};
        
        document.getElementById('machine-id').textContent = machine.id;

        // Initialize Cytoscape
        const cy = cytoscape({
            container: document.getElementById('graph'),
            boxSelectionEnabled: false,
            autounselectify: true,
            style: [
                {
                    selector: 'node',
                    style: {
                        'content': 'data(label)',
                        'text-valign': 'center',
                        'text-halign': 'center',
                        'background-color': '#fff',
                        'border-width': 2,
                        'border-color': '#333',
                        'width': 'label',
                        'height': 'label',
                        'padding': '12px',
                        'shape': 'round-rectangle',
                        'font-size': '14px'
                    }
                },
                {
                    selector: 'node.active',
                    style: {
                        'background-color': '#e7f5ff',
                        'border-color': '#228be6',
                        'color': '#228be6'
                    }
                },
                {
                    selector: 'node.initial',
                    style: {
                        'border-width': 4
                    }
                },
                {
                    selector: 'node.final',
                    style: {
                        'border-style': 'double',
                        'border-width': 4
                    }
                },
                {
                    selector: 'node:parent',
                    style: {
                        'background-color': '#f8f9fa',
                        'border-color': '#adb5bd',
                        'text-valign': 'top',
                        'text-halign': 'center',
                        'padding': '20px'
                    }
                },
                {
                    selector: 'edge',
                    style: {
                        'curve-style': 'bezier',
                        'width': 2,
                        'target-arrow-shape': 'triangle',
                        'line-color': '#999',
                        'target-arrow-color': '#999',
                        'label': 'data(label)',
                        'font-size': '11px',
                        'text-rotation': 'autorotate',
                        'text-background-color': '#fff',
                        'text-background-opacity': 1,
                        'text-background-padding': '2px'
                    }
                }
            ],
            layout: {
                name: 'dagre',
                rankDir: 'TB',
                padding: 50
            }
        });

        // Build Graph elements
        const elements = [];
        
        // Add nodes
        Object.values(machine.states).forEach(state => {
            const node = {
                data: {
                    id: state.id,
                    label: state.id,
                    parent: state.parent || undefined
                },
                classes: []
            };
            
            if (state.type === 'final') node.classes.push('final');
            if (machine.initial === state.id) node.classes.push('initial');
            
            elements.push(node);
        });

        // Add edges
        Object.values(machine.states).forEach(state => {
            if (state.transitions) {
                state.transitions.forEach(t => {
                    elements.push({
                        data: {
                            source: state.id,
                            target: t.target,
                            label: t.event + (t.guard ? ' [' + t.guard + ']' : '')
                        }
                    });
                });
            }
        });

        cy.add(elements);
        cy.layout({ name: 'dagre', rankDir: 'TB' }).run();

        // Simulation State
        let currentState = machine.initial;
        
        // Helper to find state object
        function getState(id) {
            return machine.states[id];
        }

        // Helper to resolve initial state recursively
        function resolveInitial(stateId) {
            const state = getState(stateId);
            if (state && state.type === 'compound' && state.initial) {
                return resolveInitial(state.initial);
            }
            return stateId;
        }

        currentState = resolveInitial(currentState);

        function updateUI() {
            // Update Sidebar
            document.getElementById('current-state').textContent = currentState;
            
            const eventsList = document.getElementById('events-list');
            eventsList.innerHTML = '';

            // Find transitions for current state (bubbling up)
            let activeState = getState(currentState);
            let transitions = [];
            
            while (activeState) {
                if (activeState.transitions) {
                    activeState.transitions.forEach(t => {
                        transitions.push({ ...t, source: activeState.id });
                    });
                }
                if (activeState.parent) {
                    activeState = getState(activeState.parent);
                } else {
                    activeState = null;
                }
            }

            if (transitions.length === 0) {
                const div = document.createElement('div');
                div.textContent = "No events (Final state?)";
                div.style.color = "#999";
                div.style.fontStyle = "italic";
                eventsList.appendChild(div);
            } else {
                transitions.forEach(t => {
                    const btn = document.createElement('button');
                    const label = t.event + (t.guard ? ' [' + t.guard + ']' : '');
                    
                    btn.innerHTML = '<div class="event-name">' + label + '</div><div class="target-name">→ ' + t.target + '</div>';
                    
                    btn.onclick = () => {
                        // Simple transition logic (resolve target)
                        // Note: detailed LCA logic is missing here, this is a visualization approximation
                        const target = getState(t.target);
                        if (target) {
                            if (target.type === 'history') {
                                // Simple history fallback for visualizer
                                currentState = resolveInitial(target.historyDefault || machine.initial);
                            } else {
                                currentState = resolveInitial(t.target);
                            }
                            updateUI();
                        }
                    };
                    eventsList.appendChild(btn);
                });
            }

            // Update Graph
            cy.nodes().removeClass('active');
            
            // Highlight current state and ancestors
            let current = currentState;
            while (current) {
                cy.getElementById(current).addClass('active');
                const s = getState(current);
                current = s ? s.parent : null;
            }
        }

        // Init
        updateUI();

    </script>
</body>
</html>`))
