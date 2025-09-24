# Contributing Guidelines

## Welcome to the JioTV Go Project

We're thrilled that you're considering contributing to our project. Before you get started, please take a moment to review the following guidelines to help make your contribution process as smooth as possible.

## Table of Contents

- [Contributing Guidelines](#contributing-guidelines)
  - [Welcome to the JioTV Go Project](#welcome-to-the-jiotv-go-project)
  - [Table of Contents](#table-of-contents)
  - [Code of Conduct](#code-of-conduct)
  - [How Can I Contribute?](#how-can-i-contribute)
    - [Reporting Bugs](#reporting-bugs)
    - [Suggesting Enhancements](#suggesting-enhancements)
    - [Code Contribution](#code-contribution)
    - [Documentation](#documentation)
  - [Pull Request Process](#pull-request-process)
  - [Community](#community)

## Code of Conduct

Please review our [Code of Conduct](CODE_OF_CONDUCT.md) to understand the standards of behavior expected in our community.

## How Can I Contribute?

### Development Setup

Before contributing code, make sure you have the required tools installed:

1. **Go** (version specified in go.mod) - [Download here](https://golang.org/dl/)
2. **Node.js** (16+) - [Download here](https://nodejs.org/)
3. **Git** - For version control

**Quick Setup:**
```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/jiotv_go.git
cd jiotv_go

# Install dependencies
go mod tidy
cd web && npm ci && cd ..

# Build the project
go build -o build/jiotv_go .
cd web && npm run build && cd ..

# Run tests
go test ./...
cd web && npm test && cd ..
```

### Reporting Bugs

If you encounter a bug while using our project, please open an issue on our [issue tracker](https://github.com/jiotv-go/jiotv_go/issues/) with a detailed description of the problem, steps to reproduce, and your system information.

### Suggesting Enhancements

If you have an idea for an enhancement or a new feature, feel free to open an issue on our [issue tracker](https://github.com/jiotv-go/jiotv_go/issues/). Be sure to provide a clear description of your proposal and why it would be valuable.

### Code Contribution

1. Fork the project repository.
2. Create a new branch for your feature or bug fix: `git checkout -b feature/my-new-feature` or `git checkout -b bugfix/issue-description`.
3. Make your changes and test them thoroughly.
4. Commit your changes with clear, concise commit messages following [conventional commit format](https://www.conventionalcommits.org/):
   - `feat:` for new features
   - `fix:` for bug fixes
   - `docs:` for documentation updates
   - `test:` for adding or updating tests
   - `chore:` for maintenance tasks
5. Push your changes to your forked repository.
6. Open a pull request against our main repository's `develop` branch.

### Documentation

Improvements to project documentation are also welcome. If you see any areas that could be clarified or extended, please submit a pull request with your changes.

## Pull Request Process

Before submitting a pull request, ensure the following:

- Your code follows our coding guidelines and existing patterns.
- Your commit messages are clear and follow our guidelines.
- You have added or updated tests where necessary.
- All existing tests pass (`go test ./...` for backend, `npm test` for frontend).
- The documentation is updated if needed.
- For frontend changes, ensure CSS is built with `npm run build`.

Our team will review your pull request, and once approved, it will be merged. Thank you for your contribution!

## Community

Join our community on [Community Platform/Chat Room](https://telegram.me/jiotv_go_chat) to discuss ideas, ask questions, or get help.

We appreciate your interest in contributing to the JioTV Go project.
