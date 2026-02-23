# Design Doc: Enhance grava init and add grava config

## Overview
This design aims to improve the "first-run" experience of Grava by making `grava init` a truly one-step setup and providing a `grava config` command for inspection.

## Goals
1.  `grava init` should initialize a local Dolt database in `.grava/dolt`.
2.  `grava init` should automatically find an available port if 3306 is taken.
3.  `grava init` should start the Dolt server in the background.
4.  `grava config` should display the current effective configuration.

## Architecture

### 1. `grava init` Logic
- **Dolt Init**:
    ```go
    if !doltDirExists(".grava/dolt/.dolt") {
        exec.Command("dolt", "init").Run()
    }
    ```
- **Port Detection**:
    ```go
    func findAvailablePort(startPort int) int {
        for port := startPort; port < startPort + 100; port++ {
            ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
            if err == nil {
                ln.Close()
                return port
            }
        }
        return -1
    }
    ```
- **Server Start**:
    - Build a `dolt sql-server` command with `--port` and `--host=0.0.0.0`.
    - Run it in the background using `cmd.Start()`.
    - Redirect output to a log file (e.g., `.grava/dolt.log`).

### 2. `grava config` Logic
- Utilize `viper.AllSettings()` or specific keys to show:
    - `db_url`
    - `actor`
    - `agent_model`
    - `Config file used` (from `viper.ConfigFileUsed()`)

## Data Flow
1. User runs `grava init`.
2. `init` finds port (e.g., 3307).
3. `init` writes `db_url: root@tcp(127.0.0.1:3307)/dolt?parseTime=true` to `.grava/config.yaml`.
4. `init` starts Dolt server on port 3307.

## Testing Strategy
- **Manual Verification**:
    - Run in `/example` directory.
    - Check if process is running (`ps aux | grep dolt`).
    - Check if `config.yaml` is generated correctly.
    - Run `grava config` to verify.
    - Run `grava list` to ensure connection works.
