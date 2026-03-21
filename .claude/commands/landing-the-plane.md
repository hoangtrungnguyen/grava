---
description: Complete a task by running tests, creating commits, and updating tracker
trigger: When user runs '/landing-the-plane' or asks to finalize/complete a ticket
---

# Landing the Plane

Finalize and close out a ticket properly after implementation is complete.

## Your Task

Execute the following checklist to properly close out the current ticket:

### 1. ✅ Run Tests
- Execute the test suite: `go test ./...`
- Run any relevant integration tests
- Verify all tests pass
- If tests fail, fix issues before proceeding

### 2. 🏗️ Build Verification (if applicable)
- Run build commands to verify compilation
- Check for any build warnings or errors
- For Go: `go build ./...`

### 3. 📝 Update Tracker
- Identify the current ticket being worked on
- Update the tracker file in `tracker/[TICKET-ID].md`:
  - Set status to 'Completed' or 'Done'
  - Add completion date
  - Summarize what was implemented
  - Note any architectural decisions or trade-offs

### 4. 💾 Create Commit
- Stage all relevant files
- Create a descriptive commit message following project conventions
- Reference the ticket ID in the commit message
- Include co-author tag: `Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>`

### 5. 📊 Summary Report
Present a completion summary to the user:
```
## ✅ Ticket [ID] Completed

**What was done:**
- [Brief list of changes]

**Files modified:**
- [List key files]

**Tests:** ✅ Passing
**Commit:** [commit hash or "ready to commit"]
**Tracker:** Updated

**Next steps:**
- Review the changes
- Push to remote (if ready)
- Move to next ticket
```

### 6. 🔄 Ask About Next Steps
- "Would you like to push these changes?"
- "Shall we move to the next ticket? I can run /are-u-ready to check."
