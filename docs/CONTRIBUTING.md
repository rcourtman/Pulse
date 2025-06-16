# Contributing to Pulse

Thank you for your interest in contributing to Pulse! We appreciate your help. Here are some guidelines to follow:

## Branch Strategy

Pulse uses a two-branch workflow:
- **`main`** - Stable releases only (protected)
- **`develop`** - Daily development work (default working branch)

All contributions should target the `develop` branch.

## Reporting Bugs

- Please ensure the bug was not already reported by searching on GitHub under [Issues](https://github.com/rcourtman/Pulse/issues).
- If you're unable to find an open issue addressing the problem, [open a new one](https://github.com/rcourtman/Pulse/issues/new). Be sure to include a **title and clear description**, as much relevant information as possible, and a **code sample or an executable test case** demonstrating the expected behavior that is not occurring.
- Use the "Bug Report" issue template if available.

## Suggesting Enhancements

- Open a new issue using the "Feature Request" template.
- Clearly describe the enhancement you are proposing and why it would be beneficial.
- Provide examples or mockups if possible.

## Pull Requests

### Getting Started
1. **Fork the repository** and clone your fork locally
2. **Create your branch from `develop`**: `git checkout -b feature/your-feature develop`
3. **Set up development environment**:
   ```bash
   npm install
   npm run build:css
   npm run dev  # Starts development server with hot reload
   ```

### Development Workflow
- **Local testing**: Your changes will show with dynamic RC versions (e.g., "3.24.0-rc5")
- **Version display**: RC versions increment automatically with each commit
- **No version management needed**: The system handles versioning automatically

### Before Submitting
- Ensure your code adheres to the project's existing style
- Follow existing patterns and conventions in the codebase
- Test your changes thoroughly in a development environment
- Verify your changes work with both Docker and non-Docker deployments
- Check that CSS builds correctly: `npm run build:css`

### Submitting Your Pull Request
1. **Push to your fork**: `git push origin feature/your-feature`
2. **Create a pull request** targeting the `develop` branch
3. **Provide a clear description** of what your changes do
4. **Reference any related issues** in your PR description

### After Submission
- We will review your pull request and provide feedback
- Your changes will automatically get RC releases for testing when merged to `develop`
- Once tested and approved, changes will be included in the next stable release

## Release Candidate Testing

When your PR is merged to `develop`:
- **Automatic RC creation**: A new RC release is created automatically
- **Docker images**: Multi-arch Docker images are built and published
- **Version tracking**: RC versions increment automatically (rc1, rc2, rc3...)
- **Testing opportunity**: Community can test your changes before stable release

Thank you for your contribution! 