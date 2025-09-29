

```mermaid
    A[CLI entrypoint: main.go] --> B[Analyzer]
    B --> C[Scanner]
    C --> D[Workflow Registry]
    C --> E[Detectors]
    E -->|rules.yaml| F[Config Loader]

    D --> E
    E --> G[Issues]
    G --> A
```
