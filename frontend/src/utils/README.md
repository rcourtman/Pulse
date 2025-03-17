# Search Implementation

## Formal Specification Based Search

This search implementation follows a formal specification approach to ensure robustness, consistency, and correctness. The search logic has been designed to be systematic and predictable, without relying on special case handling.

### Core Design Principles

1. **Taxonomic Structure**: The search logic follows a clear taxonomy of search operations:
   - Column-specific searches (column:value)
   - Metric comparisons with operators (>, <, =)
   - Standard role terminology (primary, secondary, shared)
   - Type and status keywords (vm, ct, running, stopped)
   - Single character searches (for any text field)
   - Numeric ID exact matches 
   - General text search across all fields

2. **No Special Case Handling**: No "priority" or special handling for specific terms like 'pri'. All terms are processed through the same clean pipeline.

3. **Proper Single Character Handling**: Single character searches (like 'p', 's') match any text containing those characters, ensuring consistent behavior.

4. **Predictable Behavior**: The search execution follows a clear, deterministic path without shortcuts or bypasses.

### Search Logic Flow

1. **Column-specific searches**: First check if the term contains a colon (e.g., `role:primary`, `status:running`).
2. **Metric comparisons**: Check for resource expressions with operators (e.g., `cpu>50`, `memory<80`).
3. **Standard role terminology**: Match exact role terms like 'primary', 'pri', 'secondary', 'sec'.
4. **Type and status keywords**: Handle standard VM/container type and status terms.
5. **Single character searches**: For single character terms, search across all text fields.
6. **Numeric ID searches**: Handle IDs with special numeric matching rules.
7. **Full text search**: As a fallback, search all text fields.

### Execution Instructions

Run the test suite to verify search functionality works as expected:

```bash
node frontend/src/utils/runSearchTests.js
```

The test suite validates comprehensive search patterns:
- Basic text searches
- Role-specific searches ('pri', 'sec', 'primary', 'secondary')
- Column-based searches ('role:pri', 'type:vm', etc.)
- Single character searches ('p', 's', 'v', etc.)
- Combined search terms with AND logic

### Search Term Guidelines

For consistent results, use standard search terms:
- Role searches: 'primary', 'pri', 'secondary', 'sec'
- Type searches: 'vm', 'ct', 'container' 
- Status searches: 'running', 'stopped', 'paused', 'suspended'
- Column-specific: 'role:primary', 'status:running', etc.
- Metrics: 'cpu>80', 'memory<50', etc.
- Single characters: Any letter for a text-based search 