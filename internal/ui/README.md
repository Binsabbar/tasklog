# Manual Testing Guide for UI Prompts

This document provides manual testing procedures for the interactive UI prompts in the tasklog application. Since the UI layer will be refactored with a different library in the future, automated tests are not included.

## Prerequisites

Build the application before testing:

```bash
go build -o /tmp/tasklog
```

---

## Test Scenarios

### 1. Time Input Retry Logic (PR #49)

**Feature**: Invalid time expressions should prompt for retry, not exit the application.

#### Test 1.1: Invalid Time Spent Format

```bash
/tmp/tasklog log
```

**Steps**:

1. Select or enter a task
2. When prompted "Enter time spent", type: `abc`
3. **Expected**: Error message displayed, prompt repeats
4. Type: `9` (missing am/pm)
5. **Expected**: Error message displayed, prompt repeats
6. Type: `2h 30m`
7. **Expected**: Proceeds to next step

**Pass Criteria**: ✅ Application does not exit on invalid input, allows retry

#### Test 1.2: Invalid Start Time Format

```bash
/tmp/tasklog log
```

**Steps**:

1. Complete task and time spent selection
2. Choose "No" when asked "Log for current time?"
3. When prompted "When did you work on this?", type: `9`
4. **Expected**: Error message displayed, prompt repeats
5. Type: `yesterday at 9am`
6. **Expected**: Proceeds to confirmation

**Pass Criteria**: ✅ Application does not exit on invalid input, allows retry

#### Test 1.3: Ctrl+C Exit

```bash
/tmp/tasklog log
```

**Steps**:

1. At any prompt, press `Ctrl+C`
2. **Expected**: Application exits cleanly with no error

**Pass Criteria**: ✅ Clean exit on Ctrl+C

---

### 2. Task Selection Retry Logic (Current PR)

**Feature**: "Task not found" errors should prompt for retry. API failures should exit immediately.

#### Test 2.1: CLI Flag Mode - No Retry (Fail Fast)

```bash
/tmp/tasklog log -t INVALID-123
```

**Steps**:

1. **Expected**: Error message "failed to fetch task INVALID-123: API request failed with status 404"
2. **Expected**: Application exits immediately (NO retry prompt)

**Pass Criteria**: ✅ Exits immediately, does NOT prompt for retry

**Rationale**: CLI flags should fail fast for predictability and scriptability.

---

#### Test 2.2: Interactive Search with Retry

```bash
/tmp/tasklog log
```

**Steps**:

1. Select "Search for a task"
2. Enter an invalid task key
3. **Expected**: If task not found, shows error and prompts for retry
4. Enter a valid task key
5. **Expected**: Proceeds with the valid task

**Pass Criteria**: ✅ Retries on "not found" in search flow

#### Test 2.3: API Failure (No Retry)

**Setup**: Temporarily break API access (e.g., invalid credentials in config)

```bash
/tmp/tasklog log -t PROJ-123
```

**Steps**:

1. **Expected**: Application exits with API error (e.g., "401 Unauthorized" or "request failed")
2. **Expected**: Does NOT prompt for retry

**Pass Criteria**: ✅ Exits immediately on API errors, no retry

**Cleanup**: Restore valid credentials

#### Test 2.4: Ctrl+C During Task Retry

```bash
/tmp/tasklog log -t INVALID-123
```

**Steps**:

1. Wait for "Enter task key:" prompt
2. Press `Ctrl+C`
3. **Expected**: Application exits cleanly

**Pass Criteria**: ✅ Clean exit on Ctrl+C during retry

---

### 3. Other Prompts (No Retry Logic)

These prompts use `survey.Required` validator which handles empty input automatically.

#### Test 3.1: Empty Comment

```bash
/tmp/tasklog log
```

**Steps**:

1. Complete task, time, and start time selection
2. When prompted "Enter a description", press Enter (empty)
3. **Expected**: survey shows error inline, re-prompts automatically
4. Enter a comment
5. **Expected**: Proceeds to confirmation

**Pass Criteria**: ✅ survey.Required handles empty input

#### Test 3.2: Confirmation Prompt

```bash
/tmp/tasklog log
```

**Steps**:

1. Complete all inputs
2. At "Log this time entry?" prompt, press Enter (default Yes)
3. **Expected**: Logs the entry
4. Run again and select "No"
5. **Expected**: Cancels without logging

**Pass Criteria**: ✅ Confirmation works as expected

---

## Regression Testing

After any changes to UI prompts, run through all scenarios to ensure:

- Retry logic works where implemented
- Ctrl+C exits cleanly from all prompts
- API errors are distinguished from validation errors
- User experience is smooth and intuitive

---

## Test Results Template

Use this template to document test results:

```
Date: YYYY-MM-DD
Tester: [Your Name]
Build: [commit hash or version]

| Test ID | Description | Result | Notes |
|---------|-------------|--------|-------|
| 1.1 | Invalid time spent | ✅ PASS | |
| 1.2 | Invalid start time | ✅ PASS | |
| 1.3 | Ctrl+C exit | ✅ PASS | |
| 2.1 | Invalid task retry | ✅ PASS | |
| 2.2 | Task search retry | ✅ PASS | |
| 2.3 | API failure no retry | ✅ PASS | |
| 2.4 | Ctrl+C during retry | ✅ PASS | |
| 3.1 | Empty comment | ✅ PASS | |
| 3.2 | Confirmation | ✅ PASS | |
```

---

## Known Limitations

1. **No automated tests**: UI prompts are tested manually only
2. **survey library**: Current implementation uses `AlecAivazis/survey/v2`, which will be replaced in future refactoring
3. **Error message patterns**: The `isTaskNotFoundError()` function relies on string matching, which may need updates if Jira API error format changes

---

## Future Improvements

When refactoring the UI layer:

- Consider adding automated tests with the new library
- Implement more robust error type checking instead of string matching
- Add configurable retry limits to prevent infinite loops
- Improve error messages with suggestions for common mistakes
