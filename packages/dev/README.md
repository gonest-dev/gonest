# GoNest Dev Tool (Hot Reload)

The `dev` package provides a hot reload utility for GoNest applications. It watches for file changes and automatically rebuilds and restarts your application.

## Features

- **Recursive Watching**: Watches all `.go` and `.env` files in your project.
- **Auto-Rebuild**: Triggers `go build` automatically on changes.
- **Graceful Restart**: Stops the previous process before starting the new one.
- **Dev Mode**: Sets `GONEST_DEV_MODE=true` environment variable.

## Usage

### Command Line

```bash
# From the root of your project
go run github.com/gonest-dev/gonest/packages/dev ./main.go [args...]
```

### VS Code Integration

#### `tasks.json`

Add a task to run the dev tool:

```json
{
    "version": "2.0.0",
    "tasks": [
        {
            "label": "gonest: dev",
            "type": "shell",
            "command": "go run github.com/gonest-dev/gonest/packages/dev ./main.go",
            "group": "test",
            "presentation": {
                "reveal": "always",
                "panel": "new"
            }
        }
    ]
}
```

#### `launch.json`

To debug with hot reload (using the dev tool as a wrapper):

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "GoNest: Hot Reload",
            "type": "go",
            "request": "launch",
            "mode": "exec",
            "program": "${workspaceFolder}/bin/gonest-dev.exe",
            "args": ["${workspaceFolder}/main.go"],
            "cwd": "${workspaceFolder}"
        }
    ]
}
```
*Note: You may need to build the tool once using `go build -o bin/gonest-dev.exe github.com/gonest-dev/gonest/packages/dev`.*

## Configuration

The tool ignores common directories like `.git`, `node_modules`, `vendor`, and `_tools`.
