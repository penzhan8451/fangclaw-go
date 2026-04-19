# FangGo Workflow Examples

This directory contains comprehensive examples of workflow configurations for fanggo, showcasing both simple patterns and real-world complex scenarios.

## Directory Structure

```
examples/workflow-examples/
├── simple/                          # Simple workflow examples
│   ├── 01-simple-sequential-pipeline.json
│   ├── 02-conditional-workflow.json
│   ├── 03-fan-out-parallel.json
│   ├── 04-error-handling-with-retry.json
│   └── 05-variable-passing.json
├── complex/                         # Complex real-world examples
│   ├── 01-software-release-workflow.json
│   ├── 02-content-creation-pipeline.json
│   ├── 03-market-research-report.json
│   ├── 04-technical-documentation-workflow.json
│   ├── 05-data-analysis-pipeline.json
│   └── 06-customer-support-ticket-handler.json
└── README.md                         # This documentation
```

## Workflow Concepts

### Step Modes

- **sequential**: Standard linear execution - one step after another
- **fan_out**: Parallel execution of multiple steps simultaneously
- **collect**: Collect outputs from previous steps
- **conditional**: Execute a step only if a specific condition is met in the previous output
- **loop**: Repeat a step multiple times until a condition is met or max iterations reached

### Error Modes

- **fail**: Stop the entire workflow if the step fails
- **skip**: Continue the workflow even if the step fails
- **retry**: Automatically retry failed steps up to N times

### Variable Reference

- `{{input}}`: The original workflow input
- `{{variable_name}}`: Custom variable defined with `output_var`
- `{{steps.step_name.output}}`: Output from a specific step

## Simple Examples

### 01-simple-sequential-pipeline.json
A basic two-step workflow demonstrating sequential execution:
- Step 1: Analyze input using an analyst agent
- Step 2: Summarize the analysis using a writer agent

### 02-conditional-workflow.json
Demonstrates conditional execution based on content analysis:
- Analyzes content for errors
- Only runs the fix step if "HAS_ERROR" is detected
- Provides final output regardless of whether fixes were needed

### 03-fan-out-parallel.json
Shows parallel execution with fan_out mode:
- Runs technical and business analyses simultaneously
- Synthesizes both perspectives into a single report

### 04-error-handling-with-retry.json
Demonstrates different error handling strategies:
- Retry mode with max 3 attempts
- Skip mode for non-critical steps
- Fail mode for essential steps

### 05-variable-passing.json
Shows how to pass variables between steps using `output_var`:
- Extracts topics and stores in "topics" variable
- Uses "topics" variable for research
- Uses research to write final report

## Complex Examples

### 01-software-release-workflow.json
Complete software release pipeline:
- Code review with retry for reliability
- Parallel security and performance analysis
- Documentation generation
- Release notes compilation

**Use Case**: Automating software quality assurance and release processes

### 02-content-creation-pipeline.json
End-to-end content creation workflow:
- Topic research
- Outline creation
- Draft writing
- Content review
- Conditional revision based on feedback
- Final formatting

**Use Case**: Content marketing, blog post generation, article writing

### 03-market-research-report.json
Comprehensive market research generator:
- Competitor analysis
- Market trends identification
- Customer insights
- SWOT analysis
- Strategic recommendations
- Final report compilation

**Use Case**: Business intelligence, market entry strategy, competitive analysis

### 04-technical-documentation-workflow.json
Technical documentation creation from code:
- Code analysis and structure understanding
- API reference documentation
- Getting started guide
- Step-by-step tutorials
- Troubleshooting guide
- Complete documentation assembly

**Use Case**: Developer documentation, API docs, library documentation

### 05-data-analysis-pipeline.json
Comprehensive data analysis workflow:
- Data quality assessment
- Data cleaning strategy
- Exploratory data analysis
- Statistical analysis
- Visualization planning
- Business insights extraction
- Strategic recommendations
- Final report compilation

**Use Case**: Business analytics, data science, market research

### 06-customer-support-ticket-handler.json
Automated customer support ticket processing:
- Ticket classification (category, priority, urgency)
- Issue analysis and root cause identification
- Urgency check with conditional escalation
- Response drafting
- Response quality review
- Conditional revision
- Final response delivery

**Use Case**: Customer support automation, helpdesk management, ticket triage

## Usage

You can create a workflow through dashboard or CLI:
`go build -o fanggo ./cmd/fangclaw-go`

### Creating a Workflow (CLI command)

```bash
# Start the daemon if not running
fanggo start

# Create a workflow from a file
fanggo workflow create examples/workflow-examples/simple/01-simple-sequential-pipeline.json
```

### Listing Workflows

```bash
fanggo workflow list
```

### Running a Workflow

```bash
# Run a workflow with input
fanggo workflow run <workflow-id> "Your input content here"
```

### Getting Workflow Status

```bash
# Check status of a specific run
fanggo workflow status <run-id>
```

### Deleting a Workflow

```bash
fanggo workflow delete <workflow-id>
```

## Best Practices

1. **Agent Selection**: Choose appropriate agents for each step (analyst for analysis, writer for content, coder for code, researcher for research)

2. **Error Handling**:
   - Use "retry" for flaky operations (3 retries is usually sufficient)
   - Use "skip" for non-critical steps that shouldn't block the workflow
   - Use "fail" for critical steps that must succeed

3. **Timeout Configuration**:
   - Simple analysis: 60-120 seconds
   - Complex research/writing: 300-600 seconds
   - Code-related tasks: 300+ seconds

4. **Variable Passing**:
   - Use descriptive variable names
   - Consider using `output_var` for intermediate results that will be reused
   - Reference step outputs with `{{steps.step_name.output}}`

5. **Parallel Execution**:
   - Use "fan_out" mode for independent tasks that can run in parallel
   - Group related parallel tasks together
   - Follow with a collect or sequential step to synthesize results

6. **Conditional Logic**:
   - Keep conditions simple and specific (e.g., "HAS_ERROR", "NEEDS_IMPROVEMENT")
   - Make sure previous steps explicitly include the condition keyword
   - Test conditions thoroughly

## JSON Schema Reference

### Workflow

```json
{
  "id": "unique-workflow-id",
  "name": "Human-readable name",
  "description": "What this workflow does",
  "steps": [
    // Array of WorkflowStep objects
  ]
}
```

### WorkflowStep

```json
{
  "name": "step-name",
  "agent": {
    "name": "agent-name"
    // OR
    "id": "agent-id"
  },
  "prompt_template": "Prompt with {{variables}}",
  "mode": "sequential",
  "timeout_secs": 120,
  "error_mode": "fail",
  "output_var": "variable_name"
}
```

### Mode Types

```json
// Sequential (string shorthand)
"mode": "sequential"

// Conditional
"mode": {
  "type": "conditional",
  "condition": "KEYWORD"
}

// Loop
"mode": {
  "type": "loop",
  "max_iterations": 5,
  "until": "DONE"
}
```

### Error Mode Types

```json
// Fail (string shorthand)
"error_mode": "fail"

// Skip (string shorthand)
"error_mode": "skip"

// Retry
"error_mode": {
  "type": "retry",
  "max_retries": 3
}
```

## More Information

- **Code Implementation**: `/internal/kernel/workflow.go` - Workflow engine
- **Type Definitions**: `/internal/types/workflow.go` - Data structures
- **CLI Commands**: `/cmd/fangclaw-go/commands/workflow.go` - Command-line interface
- **Default Templates**: Built-in templates in workflow.go `loadDefaultTemplates()`

## Tips for Custom Workflows

1. Start with simple examples and build complexity incrementally
2. Test each step individually before combining into a workflow
3. Use descriptive step names for better debugging
4. Consider using `output_var` for intermediate results you want to inspect
5. Monitor workflow runs to identify bottlenecks or failures
6. Adjust timeout values based on actual execution times
7. Document your workflows with clear descriptions
