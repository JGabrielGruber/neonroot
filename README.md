# NeonRoot

A lightweight, portable workspace manager for ephemeral development environments.

**Philosophy**: Complex simplicity — plug briefly, work untethered.

## Quick Start
```bash
go run cmd/neonroot/main.go load <pod>
```

Version: **0.0.1**
EOF
```

### 2. Initialize Git and First Commit

```bash
git init
git add .
git commit -m "Initial project structure (v0.0.1)

- Core directory layout
- Go module initialized
- Base Arch Containerfile placeholder
- Example repo structure
- Main CLI entry point
- Documentation skeleton"
```

### 3. (Optional but Recommended) Add a .gitignore

```bash
cat > .gitignore << 'EOF'
# Go
*.exe
*.exe~
*.dll
*.so
*.dylib

# Build
/bin/
/dist/

# Temporary
/tmp/
/tmp-dev-hot/

# OS
.DS_Store
Thumbs.db

# Editor
.vscode/
.idea/
*.swp
*.swo
