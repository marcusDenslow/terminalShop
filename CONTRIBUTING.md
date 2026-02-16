# Contributing with Claude Code

This project is designed to be built incrementally with AI assistance (Claude Code). This guide explains how to effectively collaborate with Claude Code on this project.

## Quick Start for Claude Code Sessions

When starting a new Claude Code session, provide this context:

```
I'm working on a terminal-based coffee shop app in Go. Please read README.md and MODELS.md to understand the project structure, current status, and technical standards before making any changes.
```

## Project Documents - Read These First!

1. **README.md** - Project overview, roadmap, current status, and what to build next
2. **MODELS.md** - Technical specs, architecture, code standards, error handling
3. **terminal-shop-source/** - Reference implementation of terminal.shop (when stuck)

## How to Ask Claude Code for Help

### ✅ Good Prompts

**When starting a new feature:**
```
I want to implement Phase 1, Task 1.1 from the README - adding the account page view.
Can you check the current model.go structure and add the account view following the
patterns already established for shop and cart views?
```

**When stuck on something:**
```
The cart isn't syncing properly between views. Can you check the cart state management
in model.go and compare it to how terminal.shop handles this in their cart.go file?
```

**When you want to follow best practices:**
```
I need to create the API endpoint for adding items to cart. Can you implement this
following the error handling patterns and response format defined in MODELS.md?
```

### ❌ Avoid Vague Prompts

```
"Make the cart better"  // Too vague - better what? Performance? UI? Features?
"Add payment stuff"     // Be specific - which phase? What functionality exactly?
"Fix the bug"          // What bug? Where? What's the expected behavior?
```

## Workflow for Building Features

### Step 1: Identify the Task
Look at README.md and find the current phase and next unchecked task:
```
## **PHASE 1: Enhanced UI & Navigation** (Current Phase)
### Tasks
#### 1.1 Add Account Page View
- [ ] Add `viewingAccount bool` to model struct in `model.go`
```

### Step 2: Reference the Specs
Check MODELS.md for:
- Code style rules
- Error handling patterns
- Similar examples
- Architecture decisions

### Step 3: Ask Claude Code
```
I want to implement [specific task from README]. I've checked MODELS.md for
the code standards. Can you implement this following the established patterns?
```

### Step 4: Test and Verify
```
Run the app and test the new feature. If issues:
"The account view isn't showing up when I press 'a'. Can you debug this?"
```

### Step 5: Update Documentation
```
The account view is working! Can you check off the completed tasks in README.md?
```

## Common Scenarios

### Starting a New Feature

1. Read the next task in README.md
2. Check if there are related patterns in MODELS.md
3. Look at terminal-shop-source for reference
4. Ask Claude Code to implement following the patterns

**Example:**
```
I'm ready to start Phase 2 - Backend API Foundation. Let's begin with task 2.1
"Project Setup". Can you create the directory structure and initialize the Chi
router following the organization defined in MODELS.md?
```

### Debugging Issues

Always provide context:
- What you were trying to do
- What you expected
- What actually happened
- Any error messages

**Example:**
```
When I add items to the cart and switch to the cart view, the items aren't showing.
I expected to see the cart items listed. The console shows no errors. Can you check
the cart state management in model.go?
```

### Refactoring Code

Reference the style guide:

```
The buildShopView function is getting long (300+ lines). Can you refactor it following
the code organization principles in MODELS.md? Split it into smaller helper functions.
```

### Adding API Endpoints

Be specific about the endpoint:

```
I need to add the POST /api/v1/cart/items endpoint from the API specs in MODELS.md.
Can you implement this with proper error handling and validation?
```

## Using the Reference Implementation

The `terminal-shop-source/` directory contains terminal.shop's code. Use it when:

### When to Reference

1. **You need UI inspiration** - Look at their TUI components
2. **You're stuck on Bubbletea patterns** - See how they handle state
3. **You want to understand payment flow** - Check their payment.go
4. **You need form examples** - Look at how they use `huh`

### How to Ask Claude Code to Reference It

```
Can you look at how terminal.shop implements the payment form in
terminal-shop-source/packages/go/pkg/tui/payment.go and create a similar
pattern for our checkout flow?
```

```
I want to add breadcrumb navigation like terminal.shop has. Can you check their
breadcrumbs.go implementation and adapt it to our project structure?
```

## Best Practices for AI-Assisted Development

### 1. Be Incremental
Build one small feature at a time. Don't try to implement entire phases at once.

✅ "Let's add the account view to the TUI"
❌ "Implement the entire authentication system"

### 2. Verify After Each Change
Run and test the code after each feature:
```bash
go run server.go model.go
```

### 3. Keep Context Clear
Start sessions with context, reference documents frequently:
```
Working on the cart API endpoints. I've checked MODELS.md for the API spec and
error handling patterns. Ready to implement POST /api/v1/cart/items.
```

### 4. Follow the Roadmap
Stick to the phases in README.md. Don't skip ahead or work on random features.

### 5. Update Documentation
When tasks are completed, update the checkboxes in README.md:
```
Can you mark task 1.1 "Add Account Page View" as complete in README.md?
```

## Troubleshooting Common Issues

### "Claude Code made changes that don't match the style guide"
```
I noticed the error messages aren't following the format in MODELS.md (they start
with capitals). Can you fix them to match our error handling standards?
```

### "The code doesn't follow the project structure"
```
The new handler was added to main.go but according to MODELS.md it should be in
api/handlers/. Can you reorganize this to match our structure?
```

### "I want to try a different approach"
```
Instead of using the pattern from MODELS.md, I want to try [your approach].
Can you implement it this way and we'll compare?
```

## Git Workflow

### Before Committing
- [ ] Code follows MODELS.md standards
- [ ] Tests pass (when applicable)
- [ ] Documentation updated (README checkboxes)
- [ ] No sensitive data (check .gitignore)

### Commit Messages
Use conventional commits:
```bash
feat: add account page view to TUI
fix: cart items not displaying correctly
refactor: split buildShopView into helper functions
docs: update README with completed tasks
```

## Tips for Effective Sessions

1. **Start each session by providing context** about what you're working on
2. **Reference the docs** (README.md, MODELS.md) frequently
3. **Ask Claude Code to check the reference implementation** when uncertain
4. **Build incrementally** - one feature at a time
5. **Test frequently** - run the app after each change
6. **Update the README** - check off completed tasks
7. **Follow the roadmap** - stick to the phases in order

## Example Full Session Flow

```
Session start:
"I'm working on Terminal Coffee Shop. Last session we completed the cart view.
Today I want to start Phase 1, Task 1.1 - adding the account page. Can you read
README.md to see what needs to be done?"

Implementation:
"Great! Now implement the account view following the same pattern as buildShopView
and buildCartView in model.go. Add the viewingAccount bool and the keybind handling."

Testing:
"I ran it but pressing 'a' doesn't switch to account view. Can you debug?"

Completion:
"Perfect! It's working now. Can you update README.md to check off all the tasks
we completed for 1.1?"
```

## Resources

- [Bubbletea Tutorial](https://github.com/charmbracelet/bubbletea/tree/master/tutorials)
- [Lipgloss Examples](https://github.com/charmbracelet/lipgloss/tree/master/examples)
- [Chi Router Guide](https://go-chi.io/#/pages/getting_started)
- [GORM Documentation](https://gorm.io/docs/)
- [Stripe Go SDK](https://github.com/stripe/stripe-go)

---

**Remember**: The goal is to build this incrementally and learn along the way. Don't rush through phases. Take time to understand each pattern before moving on!
