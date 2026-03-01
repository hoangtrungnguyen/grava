---
trigger: model_decision
description: Refactoring rule - Do not modify test files
---

When attempt to refactoring code, do not modify test file.

After refactoring success, re-run tests for these modified files. If any tests fail, fix until all the tests pass