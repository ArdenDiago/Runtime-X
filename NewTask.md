# AI Agent Instructions: 3 New Features

## 1. "Dry-Run" Registration Mode
**Goal**: Allow validating a process definition without actually saving it permanently via POST `/api/processes`.
**Instructions**:
1. In `internal/api/handlers.go`, add a `DryRun` boolean field to the `processJSON` struct (e.g., `DryRun bool ` + "`json:\"dry_run,omitempty\"`").
2. In `CreateProcess` (`internal/api/handlers.go`), check if `body.DryRun` is true.
3. Call `s.Scheduler.Register(def)` as usual.
4. If `s.Scheduler.Register(def)` succeeds and `body.DryRun` is true:
   - immediately call `s.Scheduler.Remove(def.Name)` to roll back the registration.
   - return a successful HTTP response (e.g., `201 Created` or `200 OK`) with the resulting snapshot matching existing behavior, but the process will not be left running in the scheduler.

## 2. Automated "Restart Count" Tracker
**Goal**: Track how many times a process has been automatically restarted.
**Instructions**:
1. **Note**: A close inspection of the codebase confirms that `RestartCount` is *already* fully implemented! 
   - It exists in `internal/scheduler/types.go` inside the `ManagedProcess` and `ProcessSnapshot` structs.
   - It is correctly incremented in `monitorProcess` inside `internal/scheduler/lifecycle.go`.
   - It is serialized in `internal/api/handlers.go` via `processJSON`.
2. Please quickly verify this is indeed the case. If confirmed, no further action/modifications are needed for this specific feature.

## 3. Environment Variable Validation
**Goal**: Ensure all environment variables provided in a `ProcessDef` follow the `KEY=VALUE` format.
**Instructions**:
1. In `internal/scheduler/scheduler.go`, create a new helper function (e.g., `validateEnv(env []string) error`).
2. The helper should iterate over the environment strings, use `strings.Split(e, "=")` and verify that the key portion (`parts[0]`) is not empty. If it lacks a `=` or has an empty key, return an error indicating the format is invalid.
3. Inside the `Register` method in `internal/scheduler/scheduler.go`, call your new `validateEnv(def.Env)` helper near the top of the function (e.g., right after `validateName` is called).
4. Return the validation error if it fails. The `CreateProcess` handler in `handlers.go` will automatically forward this error as a `422 Unprocessable Entity`.

---

## Frontend Changes (All 3 Features)

### Types (`web/src/api/types.ts`)
- Added `dry_run?: boolean` field to the `ProcessJSON` interface.

### API Client (`web/src/api/client.ts`)
- Added `dryRunProcess(def)` function — sends a `POST /api/processes` with `{ ...def, dry_run: true }`.

### Process Form (`web/src/components/ProcessForm.tsx`)
- **Environment Variables**: New `<textarea>` field for entering `KEY=VALUE` pairs, one per line. Lines are split by newline and sent as the `env` string array.
- **Dry-Run Button**: An orange "Validate (Dry Run)" button next to the blue "Create" button. Calls `dryRunProcess()` and shows a green success message or red error without closing the form.
- **Restart Count**: Already displayed in `ProcessList.tsx` — no changes needed.
