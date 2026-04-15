# First Steps

After [installing](installation.md) and [configuring your editor](editor-setup.md), let's verify everything works.

## Verify Basic Features

Create a test file `test_example.py`:

```python
class Person:
    def __init__(self, name: str, age: int):
        self.name = name
        self.age = age
    
    def greet(self) -> str:
        return f"Hi, I'm {self.name}"

def main():
    person = Person("Alice", 30)
    message = person.greet()
    print(message)

if __name__ == "__main__":
    main()
```

### Test 1: Hover Information

Hover over `Person`, `greet`, or `message` - you should see type information.

**Expected**: Hover shows type signatures and documentation.

### Test 2: Go to Definition

1. Put cursor on `greet` in `person.greet()`
2. Trigger "Go to Definition" (usually F12 or Cmd/Ctrl+Click)

**Expected**: Jumps to line 6 where `def greet` is defined.

### Test 3: Completion

1. Type `person.` after the Person instance is created
2. Trigger completion (usually Ctrl+Space)

**Expected**: Shows `name`, `age`, `greet`, `__init__` as completions.

### Test 4: Diagnostics

Add an error to the file:

```python
def main():
    person = Person("Alice", "not a number")  # Type error!
```

**Expected**: Red underline on `"not a number"` with error message.

## Understanding What Rahu Can Do

### Workspace Features

Rahu indexes your entire workspace:

```python
# In utils/helpers.py
def helper_function():
    return 42

# In main.py
from utils.helpers import helper_function

result = helper_function()  # Rahu knows this returns int
```

Try hovering over `helper_function` - rahu shows it's from `utils/helpers.py`.

### Import Resolution

Rahu handles various import styles:

```python
# Absolute imports
import os.path
from collections import defaultdict

# Relative imports
from . import sibling_module
from ..parent import something

# Star imports (with __all__ awareness)
from mymodule import *
```

### Type Annotations

Rahu understands Python type hints:

```python
from typing import List, Dict, Optional

def process(items: List[str]) -> Dict[str, int]:
    result: Dict[str, int] = {}
    for item in items:
        result[item] = len(item)
    return result

# Optional types
def maybe_greet(name: Optional[str] = None) -> str:
    if name is None:
        return "Hello, stranger!"
    return f"Hello, {name}!"
```

## Common Workflows

### Refactoring

**Rename a symbol**:
1. Put cursor on `Person`
2. Trigger "Rename Symbol" (usually F2)
3. Type new name `Human`
4. All references update

**Limitation**: Rahu renames within the workspace, but not across files outside the project.

### Finding References

1. Put cursor on `greet` method
2. Trigger "Find All References" (usually Shift+F12)
3. See all call sites

### Document Outline

Open "Document Symbols" (usually Ctrl+Shift+O or Cmd+Shift+O) to see:
- Classes
- Functions  
- Variables
- Imports

## What to Expect

### ✅ Works Well

- Python 3.10-3.14 syntax
- Standard library (via typeshed stubs)
- Your workspace modules
- Most common type annotations

### ⚠️ Known Limitations

See [What's Missing](../ROADMAP.md) for complete list.

Main limitations to be aware of:
- **Lambda expressions** - Not yet supported
- **Walrus operator** (`:=`) - Not yet supported  
- **Async/await** - Not yet supported
- **Match statements** - Not yet supported

## Next Steps

Now that you've verified rahu works:

1. [Configure settings](../user-guide/configuration.md) - Customize behavior
2. [Learn about all features](../user-guide/features.md) - Deep dive into capabilities
3. [Read troubleshooting tips](../user-guide/troubleshooting.md) - Fix common issues
4. [Understand the architecture](../architecture/overview.md) - How it works internally

Happy coding!
