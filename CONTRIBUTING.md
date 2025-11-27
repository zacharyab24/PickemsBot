# Contributing Guidelines

Thanks for your interest in contributing! 🎉  
This is a personal project, but contributions are very welcome.

Please read this document before opening issues or pull requests.

---

## 1. Code of Conduct

- Be respectful and constructive.
- Assume good intent.
- Disagreements are fine; personal attacks are not.

If there’s a problem, please open an issue describing it calmly and clearly.

---

## 2. How to Contribute

### 2.1. Feature Requests

1. **Check existing issues** to see if your idea is already tracked.  
2. If not, **open a new issue**:
   - Use a clear, descriptive title.
   - Explain the use case and why it’s useful.
   - Add any relevant examples or pseudo-code.

> **Important:** If you plan to implement a feature yourself, **create an issue first**, discuss the approach if needed, then open a PR referencing that issue.

### 2.2. Bug Reports

When reporting a bug, please include:

- Steps to reproduce
- Expected behavior
- Actual behavior
- Environment (OS, Go version, etc.)
- Any relevant logs or stack traces

---

## 3. Pull Requests

### 3.1. Before You Open a PR

- Make sure there is an **issue** describing the change.
- **Discuss large changes** in the issue first to avoid wasted work.
- Ensure your branch is up to date with the default branch (e.g. `main`).

### 3.2. PR Requirements

All pull requests should:

1. **Reference an issue**  
   - In the PR description, include something like: `Closes #123` or `Fixes #123`.
2. **Include tests for new behavior**  
   - New features and bug fixes must include test coverage.
3. **Pass all checks**  
   - CI must pass (format, build, vet, lint, static analysis, tests, coverage, etc.).
4. **Include documentation updates where relevant**  
   - Update README, docs, or comments if behavior or APIs change.
5. **Follow coding style and conventions** used in the project.

---

## 4. Testing & Coverage

This project aims for **at least 80% test coverage** overall.

The CI pipeline runs:

- `go test ./... -coverprofile=coverage.out -v`
- A coverage gate that **fails the build if total coverage is below 80%**.

Guidelines:

- New code **must be covered by tests**.
- If you touch existing code without tests, consider adding tests for it.
- PRs that significantly reduce coverage may be rejected or requested to add more tests.

Add tests for:

- New functions, methods, or modules.
- New branches in logic (if/else, error paths).
- Regression tests for fixed bugs.

---

## 5. Linting, Formatting & Static Analysis

All code must pass the automated **Go quality checks** that run in CI. The GitHub Actions workflow (`CI - Go Full Quality Check`) performs the following checks and the build will fail if any step reports an error (unless the error is explicitly discussed and accepted in the linked issue/PR):

1. **Formatting**
   - `gofmt` is run over the codebase and the build **fails if any files are not properly formatted**.
   - Before pushing, run:
     ```bash
     gofmt -w .
     ```

2. **Build verification**
   - `go build ./...` must succeed.

3. **Vet**
   - `go vet ./...` must pass without issues.

4. **Linting**
   - `golint ./...` is run and the build **fails if lint issues are reported**.
   - Fix or justify any lint warnings before opening a PR.
   - To run locally (example):
     ```bash
     go install golang.org/x/lint/golint@latest
     golint ./...
     ```

5. **Static analysis**
   - `staticcheck ./...` is run and must pass.
   - To run locally:
     ```bash
     go install honnef.co/go/tools/cmd/staticcheck@latest
     staticcheck ./...
     ```

6. **Vulnerability check**
   - `govulncheck ./...` is run to catch known vulnerabilities.
   - If issues are reported, please address them or explain why they are safe/acceptable in the PR description.

7. **CI workflow reference**
   - The project CI is named **`CI - Go Full Quality Check`** and includes all of the steps above plus test coverage enforcement and artifact upload.

Your PR should not introduce new warnings or errors in any of these steps.

---

## 6. AI-Generated Code

AI-generated code is **welcome**, with a few conditions:

1. **Be transparent**  
   - Clearly indicate in the PR description which parts are AI-generated.  
   - Optionally, add a brief comment near non-trivial AI-generated blocks, for example:
     ```go
     // NOTE: This function was initially generated with the help of an AI assistant.
     ```

2. **You are responsible for the code**  
   - Review the code carefully.
   - Make sure it builds, passes tests, and follows project conventions.
   - Verify there are no obvious security, performance, or licensing issues.

3. **Refine as needed**  
   - Treat AI output as a draft; simplify and clean it up where possible.

---

## 7. Commit Messages

This project uses **Conventional Commits**.

Format:
```
<type>(optional-scope): <short summary>
```

Examples:

- `feat: add user login endpoint`
- `fix: handle nil config in server`
- `chore(test): improve integration test setup`
- `docs: update contributing guidelines`
- `refactor(core): simplify request handling`

Common types:

- `feat` – New feature
- `fix` – Bug fix
- `docs` – Documentation only changes
- `style` – Formatting changes (no logic changes)
- `refactor` – Code changes that neither fix a bug nor add a feature
- `test` – Adding or updating tests
- `chore` – Build process or auxiliary tools and libraries

Guidelines:

- Use the **imperative mood** in summaries (e.g. “add X”, not “added X”).
- Keep the summary reasonably short (around ~50 characters if possible).

---

## 8. Branching

Recommended branch naming (not enforced, but helpful):

- `feat/<short-description>` for features (e.g. `feat/user-auth`)
- `fix/<short-description>` for bug fixes (e.g. `fix/panic-on-startup`)
- `chore/<short-description>` for maintenance (e.g. `chore/update-ci-pipeline`)

---

## 9. Getting Help

If you’re unsure about anything:

- Open an issue with your question or proposal.
- Or open a **Draft PR** to get early feedback on an approach.

---

Thanks again for taking the time to contribute 💜
