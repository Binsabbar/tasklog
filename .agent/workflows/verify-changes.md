---
description: Verify code changes with linting, testing, and vulnerability checks
---

1. Run the linter to catch style and potential correctness issues.
   // turbo

   ```bash
   make go-lint
   ```

2. If there are any linting errors, fix them and re-run the linter until it passes.

3. Run the test suite to ensure no regressions.
   // turbo

   ```bash
   make go-test
   ```

4. If there are any test failures, fix the code and re-run the tests until they pass.

5. Run the vulnerability checker to ensure dependencies are secure.
   // turbo

   ```bash
   make go-vulncheck
   ```

6. If any vulnerabilities are found, address them by updating dependencies or applying patches.
