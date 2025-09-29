# Cadence Workflow Linter - Design Documentation

## Table of Contents
1. [Project Overview](#project-overview)
2. [System Architecture](#system-architecture)
3. [Component Design](#component-design)
4. [Data Flow Diagrams](#data-flow-diagrams)
5. [Class/Interface Diagrams](#classinterface-diagrams)
6. [Sequence Diagrams](#sequence-diagrams)
7. [Configuration Design](#configuration-design)
8. [Detection Strategy](#detection-strategy)

## Project Overview

### Purpose
The Cadence Workflow Linter is a static analysis tool designed to identify non-deterministic or unsafe operations in Uber Cadence workflows. It ensures workflow code remains deterministic, reliable, and production-safe while avoiding false positives in activity code.

### Key Goals
- **Determinism**: Ensure workflows are deterministic and replay-safe
- **Extensibility**: Support for adding new detection rules without core changes
- **Accuracy**: Distinguish between workflow and activity code to avoid false positives
- **Configurability**: Rule-based configuration through YAML files

## System Architecture

### High-Level Architecture Diagram

```mermaid
graph TB
    subgraph "CLI Layer"
        CLI[main.go<br/>Command Line Interface]
    end
    
    subgraph "Core Analysis Engine"
        ANALYZER[Analyzer<br/>scanner.go]
        REGISTRY[Workflow Registry<br/>Call Graph Builder]
    end
    
    subgraph "Detection Engine"
        DETECTOR_FACTORY[Detector Factory]
        FUNC_DETECTOR[Function Call Detector]
        IMPORT_DETECTOR[Import Detector]
        GOROUTINE_DETECTOR[Goroutine Detector]
        CHANNEL_DETECTOR[Channel Detector]
    end
    
    subgraph "Configuration"
        CONFIG[Config Loader]
        RULES[rules.yaml]
    end
    
    subgraph "Input/Output"
        SOURCE[Go Source Files]
        ISSUES[Issues<br/>JSON/YAML Output]
    end
    
    CLI --> ANALYZER
    CLI --> CONFIG
    CONFIG --> RULES
    CONFIG --> DETECTOR_FACTORY
    ANALYZER --> REGISTRY
    ANALYZER --> DETECTOR_FACTORY
    DETECTOR_FACTORY --> FUNC_DETECTOR
    DETECTOR_FACTORY --> IMPORT_DETECTOR
    DETECTOR_FACTORY --> GOROUTINE_DETECTOR
    DETECTOR_FACTORY --> CHANNEL_DETECTOR
    REGISTRY --> FUNC_DETECTOR
    SOURCE --> ANALYZER
    FUNC_DETECTOR --> ISSUES
    IMPORT_DETECTOR --> ISSUES
    GOROUTINE_DETECTOR --> ISSUES
    CHANNEL_DETECTOR --> ISSUES
```

### Architectural Layers

1. **CLI Layer**: Entry point handling command-line arguments and output formatting
2. **Analysis Engine**: Core scanning and registry management
3. **Detection Engine**: Pluggable detectors for different violation types
4. **Configuration Layer**: YAML-based rule configuration
5. **I/O Layer**: File parsing and issue reporting

## Component Design

### 1. Main Components Overview

```mermaid
graph LR
    subgraph "main.go"
        MAIN[Main Function]
        FLAGS[Flag Parsing]
        OUTPUT[Output Formatting]
    end
    
    subgraph "analyzer/"
        SCANNER[Scanner]
        subgraph "registry/"
            WR[WorkflowRegistry]
        end
        subgraph "detectors/"
            BASE[Detector Interface]
            FUNC[FuncCallDetector]
            IMP[ImportDetector]
            GOR[GoroutineDetector]
            CHAN[ChannelDetector]
        end
    end
    
    subgraph "config/"
        LOADER[Config Loader]
        YAML[rules.yaml]
    end
    
    MAIN --> SCANNER
    MAIN --> LOADER
    SCANNER --> WR
    SCANNER --> BASE
    BASE --> FUNC
    BASE --> IMP
    BASE --> GOR
    BASE --> CHAN
    LOADER --> YAML
```

### 2. Detector Interface Design

```mermaid
classDiagram
    class Detector {
        <<interface>>
        +Visit(node ast.Node) ast.Visitor
    }
    
    class WorkflowAware {
        <<interface>>
        +SetWorkflowRegistry(reg *WorkflowRegistry)
    }
    
    class FileContextAware {
        <<interface>>
        +SetFileContext(ctx FileContext)
    }
    
    class IssueProvider {
        <<interface>>
        +Issues() []Issue
    }
    
    class FuncCallDetector {
        -rules []FunctionCallRule
        -registry *WorkflowRegistry
        -issues []Issue
        -fileContext FileContext
        +Visit(node ast.Node) ast.Visitor
        +SetWorkflowRegistry(reg *WorkflowRegistry)
        +SetFileContext(ctx FileContext)
        +Issues() []Issue
    }
    
    class ImportDetector {
        -rules []ImportRule
        -issues []Issue
        -fileContext FileContext
        +Visit(node ast.Node) ast.Visitor
        +SetFileContext(ctx FileContext)
        +Issues() []Issue
    }
    
    Detector <|.. FuncCallDetector
    Detector <|.. ImportDetector
    WorkflowAware <|.. FuncCallDetector
    FileContextAware <|.. FuncCallDetector
    FileContextAware <|.. ImportDetector
    IssueProvider <|.. FuncCallDetector
    IssueProvider <|.. ImportDetector
```

## Data Flow Diagrams

### 1. Overall Analysis Flow

```mermaid
flowchart TD
    START([Start]) --> PARSE_CLI[Parse CLI Arguments]
    PARSE_CLI --> LOAD_CONFIG[Load Configuration]
    LOAD_CONFIG --> CHECK_TARGET{Target is Directory?}
    
    CHECK_TARGET -->|Yes| SCAN_DIR[Scan Directory]
    CHECK_TARGET -->|No| SCAN_FILE[Scan Single File]
    
    SCAN_DIR --> PARSE_FILES[Parse All Go Files]
    SCAN_FILE --> PARSE_SINGLE[Parse Single File]
    
    PARSE_FILES --> BUILD_REGISTRY[Build Workflow Registry]
    PARSE_SINGLE --> BUILD_REGISTRY
    
    BUILD_REGISTRY --> CREATE_DETECTORS[Create Detector Instances]
    CREATE_DETECTORS --> ANALYZE_AST[Analyze AST with Detectors]
    
    ANALYZE_AST --> COLLECT_ISSUES[Collect Issues from Detectors]
    COLLECT_ISSUES --> FORMAT_OUTPUT{Output Format?}
    
    FORMAT_OUTPUT -->|JSON| JSON_OUT[Format as JSON]
    FORMAT_OUTPUT -->|YAML| YAML_OUT[Format as YAML]
    
    JSON_OUT --> OUTPUT[Output Results]
    YAML_OUT --> OUTPUT
    OUTPUT --> END([End])
```

### 2. Workflow Registry Building Process

```mermaid
flowchart TD
    START([Start Registry Building]) --> ITERATE_FILES[Iterate Through Parsed Files]
    ITERATE_FILES --> WALK_AST[Walk AST of Each File]
    
    WALK_AST --> CHECK_NODE{Node Type?}
    
    CHECK_NODE -->|FuncDecl| ANALYZE_FUNC[Analyze Function Declaration]
    CHECK_NODE -->|CallExpr| ANALYZE_CALL[Analyze Function Call]
    CHECK_NODE -->|Other| CONTINUE[Continue Walking]
    
    ANALYZE_FUNC --> CHECK_PARAMS[Check Parameter Types]
    CHECK_PARAMS --> WORKFLOW_CTX{workflow.Context?}
    WORKFLOW_CTX -->|Yes| ADD_WORKFLOW[Add to WorkflowFuncs]
    WORKFLOW_CTX -->|No| CHECK_ACTIVITY{context.Context?}
    
    CHECK_ACTIVITY -->|Yes| ADD_ACTIVITY[Add to ActivityFuncs]
    CHECK_ACTIVITY -->|No| CONTINUE
    
    ADD_WORKFLOW --> CONTINUE
    ADD_ACTIVITY --> CONTINUE
    
    ANALYZE_CALL --> ADD_CALL_EDGE[Add Edge to Call Graph]
    ADD_CALL_EDGE --> CONTINUE
    
    CONTINUE --> MORE_NODES{More Nodes?}
    MORE_NODES -->|Yes| WALK_AST
    MORE_NODES -->|No| MORE_FILES{More Files?}
    
    MORE_FILES -->|Yes| ITERATE_FILES
    MORE_FILES -->|No| COMPUTE_REACHABILITY[Compute Reachable Functions]
    COMPUTE_REACHABILITY --> END([Registry Complete])
```

### 3. Detection Process Flow

```mermaid
flowchart TD
    START([Start Detection]) --> GET_DETECTORS[Get Detector Instances]
    GET_DETECTORS --> SET_CONTEXT[Set Workflow Registry & File Context]
    
    SET_CONTEXT --> WALK_AST[Walk AST with Each Detector]
    WALK_AST --> DETECTOR_VISIT[Detector.Visit(node)]
    
    DETECTOR_VISIT --> CHECK_VIOLATION{Violation Detected?}
    
    CHECK_VIOLATION -->|Yes| CHECK_WORKFLOW_CONTEXT{In Workflow Context?}
    CHECK_VIOLATION -->|No| CONTINUE_WALK[Continue Walking]
    
    CHECK_WORKFLOW_CONTEXT -->|Yes| CREATE_ISSUE[Create Issue]
    CHECK_WORKFLOW_CONTEXT -->|No| CONTINUE_WALK
    
    CREATE_ISSUE --> ADD_ISSUE[Add to Issues List]
    ADD_ISSUE --> CONTINUE_WALK
    
    CONTINUE_WALK --> MORE_NODES{More Nodes?}
    MORE_NODES -->|Yes| DETECTOR_VISIT
    MORE_NODES -->|No| MORE_DETECTORS{More Detectors?}
    
    MORE_DETECTORS -->|Yes| WALK_AST
    MORE_DETECTORS -->|No| COLLECT_ISSUES[Collect All Issues]
    
    COLLECT_ISSUES --> END([Detection Complete])
```

## Class/Interface Diagrams

### 1. Core Interfaces and Structures

```mermaid
classDiagram
    class Issue {
        +File string
        +Line int
        +Column int
        +Rule string
        +Severity string
        +Message string
        +Func string
        +CallStack []string
    }
    
    class FileContext {
        +File string
        +Fset *token.FileSet
        +ImportMap map[string]string
    }
    
    class WorkflowRegistry {
        +WorkflowFuncs map[string]bool
        +ActivityFuncs map[string]bool
        +CallGraph map[string][]string
        +Visit(node ast.Node) ast.Visitor
        +IsWorkflowReachable(funcName string) bool
        +GetCallStack(from, to string) []string
    }
    
    class parsedFile {
        +filename string
        +fset *token.FileSet
        +node *ast.File
        +importMap map[string]string
    }
```

### 2. Configuration Structure

```mermaid
classDiagram
    class Rules {
        +FunctionCalls []FunctionCallRule
        +DisallowedImports []ImportRule
    }
    
    class FunctionCallRule {
        +Rule string
        +Package string
        +Functions []string
        +Severity string
        +Message string
    }
    
    class ImportRule {
        +Rule string
        +Path string
        +Severity string
        +Message string
    }
    
    Rules --> FunctionCallRule
    Rules --> ImportRule
```

## Sequence Diagrams

### 1. Main Execution Sequence

```mermaid
sequenceDiagram
    participant CLI as main.go
    participant Config as config.LoadRules
    participant Analyzer as analyzer.Scanner
    participant Registry as WorkflowRegistry
    participant Detectors as Detector[]
    
    CLI->>Config: LoadRules(rulesPath)
    Config-->>CLI: Rules
    
    CLI->>Analyzer: ScanFile/ScanDirectory(target, factory)
    
    Analyzer->>Analyzer: parseAllAndBuildRegistry(target)
    Analyzer->>Registry: NewWorkflowRegistry()
    Registry-->>Analyzer: WorkflowRegistry instance
    
    loop For each Go file
        Analyzer->>Analyzer: parseFile(path)
        Analyzer->>Registry: ast.Walk(registry, file)
    end
    
    Analyzer->>Detectors: factory() - create detector instances
    Detectors-->>Analyzer: []ast.Visitor
    
    loop For each detector
        Analyzer->>Detectors: SetWorkflowRegistry(registry)
        Analyzer->>Detectors: SetFileContext(context)
    end
    
    loop For each file
        loop For each detector
            Analyzer->>Detectors: ast.Walk(detector, file)
        end
    end
    
    loop For each detector
        Analyzer->>Detectors: Issues()
        Detectors-->>Analyzer: []Issue
    end
    
    Analyzer-->>CLI: All issues
    CLI->>CLI: Format output (JSON/YAML)
```

### 2. Detector Execution Sequence

```mermaid
sequenceDiagram
    participant Scanner as analyzer.Scanner
    participant Detector as FuncCallDetector
    participant Registry as WorkflowRegistry
    participant AST as ast.Node
    
    Scanner->>Detector: SetWorkflowRegistry(registry)
    Scanner->>Detector: SetFileContext(fileContext)
    
    Scanner->>AST: ast.Walk(detector, file)
    
    loop For each AST node
        AST->>Detector: Visit(node)
        
        alt Node is CallExpr
            Detector->>Detector: extractFunctionCall(callExpr)
            Detector->>Registry: IsWorkflowReachable(currentFunc)
            Registry-->>Detector: isReachable bool
            
            alt Is workflow reachable AND violation found
                Detector->>Detector: createIssue(node, rule)
                Detector->>Detector: addToIssues(issue)
            end
        end
        
        Detector-->>AST: Continue/Stop walking
    end
    
    Scanner->>Detector: Issues()
    Detector-->>Scanner: []Issue
```

## Configuration Design

### 1. Rules Configuration Structure

The configuration system uses YAML to define linting rules:

```yaml
# Function call rules
function_calls:
  - rule: TimeUsage
    package: time
    functions: [Now, Since, Sleep]
    severity: error
    message: "Detected time.%FUNC%() in workflow. Use workflow.Now(ctx)/workflow.Sleep(ctx) instead."

# Import rules
disallowed_imports:
  - rule: ImportRandom
    path: math/rand
    severity: warning
    message: "Importing math/rand in files with workflows is discouraged"
```

### 2. Configuration Loading Flow

```mermaid
flowchart TD
    START([Start Config Loading]) --> READ_FILE[Read YAML File]
    READ_FILE --> PARSE_YAML[Parse YAML Content]
    PARSE_YAML --> VALIDATE{Valid Structure?}
    
    VALIDATE -->|No| ERROR[Return Error]
    VALIDATE -->|Yes| CREATE_RULES[Create Rules Structure]
    
    CREATE_RULES --> VALIDATE_RULES[Validate Rule Definitions]
    VALIDATE_RULES --> RULES_VALID{Rules Valid?}
    
    RULES_VALID -->|No| ERROR
    RULES_VALID -->|Yes| RETURN_RULES[Return Rules]
    
    ERROR --> END([End with Error])
    RETURN_RULES --> END([End Successfully])
```

## Detection Strategy

### 1. Workflow Context Detection

The linter uses a sophisticated approach to determine if code is executing in a workflow context:

1. **Function Signature Analysis**: Identifies functions with `workflow.Context` as workflow functions
2. **Call Graph Construction**: Builds a graph of function calls to track reachability
3. **Reachability Analysis**: Determines if a function is reachable from a workflow function

### 2. False Positive Avoidance

```mermaid
flowchart TD
    DETECT[Potential Violation Detected] --> CHECK_CONTEXT{In Workflow Context?}
    
    CHECK_CONTEXT -->|No| IGNORE[Ignore - Activity Code]
    CHECK_CONTEXT -->|Yes| CHECK_REACHABILITY{Reachable from Workflow?}
    
    CHECK_REACHABILITY -->|No| IGNORE
    CHECK_REACHABILITY -->|Yes| CREATE_ISSUE[Create Issue]
    
    IGNORE --> END([No Issue])
    CREATE_ISSUE --> END([Issue Created])
```

### 3. Multi-File Analysis

The linter performs two-pass analysis:

1. **Pass 1**: Parse all files and build the global workflow registry
2. **Pass 2**: Run detectors with full context of the entire codebase

This approach ensures accurate detection across file boundaries and proper handling of helper functions defined in separate files.

---

This design documentation provides a comprehensive overview of the Cadence Workflow Linter's architecture, components, and design decisions. The modular design allows for easy extension with new detectors while maintaining accuracy through sophisticated workflow context analysis.