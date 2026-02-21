# PRD: One-Script Installer

## Problem Statement

Setting up a container orchestration system on a VPS requires multiple manual steps — installing dependencies, configuring services, setting up networking, and starting background processes. This is error-prone, time-consuming, and creates a high barrier to entry for solo developers who just want to run containers in production.

## Solution

A single shell script that fully installs and configures Dirigent and all its dependencies on a fresh Ubuntu/Debian VPS. After running one command, the user has a fully operational system and can immediately start deploying containers via the GUI.

## User Stories

1. As a solo developer, I want to install Dirigent with a single command, so that I can start deploying containers without manual setup steps.
2. As a developer, I want the installer to install Docker if it's not already present, so that I don't need to configure it separately before running the script.
3. As a developer, I want the installer to set up Dirigent as a systemd service, so that it starts automatically on boot and survives VPS restarts.
4. As a developer, I want the installer to create a dedicated Docker network for Dirigent, so that container networking is managed consistently.
5. As a developer, I want the installer to set up and start the integrated reverse proxy, so that containers with domains are reachable from the internet immediately after deploy.
6. As a developer, I want the installer to validate that it's running on a supported OS version, so that I get a clear error instead of a cryptic failure mid-install.
7. As a developer, I want the installer to check that it's run as root or with sudo, so that permission errors don't interrupt the setup.
8. As a developer, I want to see progress output as the installer runs, so that I know it hasn't stalled.
9. As a developer, I want the installer to print the GUI URL when it finishes, so that I can immediately open it and start deploying.
10. As a developer, I want the install to be idempotent where possible, so that re-running it on an already-configured system doesn't break anything.

## Implementation Decisions

- Shell script (bash), targeting Ubuntu 22.04+ and Debian 11+
- Script checks for root/sudo at the start and exits with a clear message if not present
- Script detects OS and version, exits with a clear error on unsupported systems
- Docker installation follows the official Docker apt repository method
- Dirigent binary is downloaded from GitHub releases
- A dedicated Docker bridge network is created for Dirigent-managed containers
- A systemd unit file is written and enabled for the Dirigent process
- The reverse proxy is started as part of the Dirigent process (not a separate service)
- Script prints a summary on completion: GUI URL, status of each setup step

## Testing Decisions

- Good tests verify observable outcomes (service running, network exists, binary present), not internal script logic
- Test: after running the script on a fresh Ubuntu 22.04 VM, the systemd service is active
- Test: after install, the GUI is reachable on the expected port
- Test: Docker network for Dirigent exists post-install
- Manual smoke test on Debian 11 and Ubuntu 24.04

## Out of Scope

- Upgrade / re-install support (v2)
- Non-Debian Linux distributions
- ARM architecture support
- Uninstall script
- Air-gapped / offline install

## Further Notes

The install experience is a key part of the product's value proposition — the time from "blank VPS" to "running container" must be under 5 minutes. The script should be readable and commented so users who want to understand what it does can do so easily.
