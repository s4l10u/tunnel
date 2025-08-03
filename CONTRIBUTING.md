# Contributing to Tunnel

Thank you for your interest in contributing to the Tunnel project! This guide will help you get started.

## 🚀 Quick Start

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR-USERNAME/tunnel.git
   cd tunnel
   ```
3. **Create a branch** for your feature:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## 🛠️ Development Setup

### Prerequisites
- Go 1.19 or later
- Docker and Docker Compose
- Make

### Build and Test
```bash
# Install dependencies
make deps

# Build binaries
make build

# Run tests
go test ./...

# Start development environment
make dev

# Test tunnel connection
make test
```

## 📝 Making Changes

### Code Style
- Follow Go conventions and best practices
- Use `gofmt` to format your code
- Add comments for exported functions and types
- Keep functions small and focused

### Security Guidelines
- Never commit secrets, tokens, or credentials
- Use secure defaults in configuration
- Validate all inputs
- Follow the principle of least privilege

### Testing
- Add tests for new features
- Ensure existing tests pass
- Test on multiple platforms when possible
- Include integration tests for critical paths

## 🔄 Submitting Changes

1. **Commit your changes**:
   ```bash
   git add .
   git commit -m "feat: add new feature description"
   ```

2. **Push to your fork**:
   ```bash
   git push origin feature/your-feature-name
   ```

3. **Create a Pull Request** on GitHub with:
   - Clear description of changes
   - Reference to any related issues
   - Screenshots/demos if applicable

### Commit Message Format
```
type(scope): description

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `test`: Adding tests
- `chore`: Maintenance tasks

**Examples:**
- `feat(client): add automatic reconnection`
- `fix(server): resolve memory leak in TCP handler`
- `docs: update installation instructions`

## 🐛 Reporting Issues

When reporting issues, please include:
- Operating system and version
- Go version
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs or error messages

## 🚀 Feature Requests

For new features:
- Check if it already exists in issues
- Describe the use case clearly
- Explain why it would be valuable
- Consider implementation complexity

## 📋 Development Areas

We welcome contributions in:

### Core Features
- Protocol improvements
- Performance optimizations
- Security enhancements
- Platform support

### Documentation
- Installation guides
- Configuration examples
- Troubleshooting guides
- API documentation

### Testing
- Unit tests
- Integration tests
- Performance benchmarks
- Security testing

### DevOps
- CI/CD improvements
- Docker optimizations
- Kubernetes manifests
- Release automation

## 🔒 Security

For security-related issues:
- **DO NOT** open public issues
- Email security concerns privately
- Allow time for fixes before disclosure

## 📚 Resources

- [Go Documentation](https://golang.org/doc/)
- [WebSocket RFC](https://tools.ietf.org/html/rfc6455)
- [systemd Service Documentation](https://www.freedesktop.org/software/systemd/man/systemd.service.html)

## 🤝 Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to uphold this code.

## ❓ Getting Help

- Check existing [issues](https://github.com/s4l10u/tunnel/issues)
- Review [documentation](README.md)
- Ask questions in discussions

Thank you for contributing! 🎉