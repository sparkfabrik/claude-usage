# Spec Delta

## Purpose

Defines how the terminal dashboard adapts its colors and text styling to where its output goes, so it reads well in a terminal and stays clean when piped or when the user opts out of colors.

## ADDED Requirements

### Requirement: Plain output when not writing to a terminal

The dashboard SHALL write no ANSI escape sequences when standard output is not a terminal, such as a pipe or a file, unless the user forces colors with `CLICOLOR_FORCE` and `NO_COLOR` is not set.

#### Scenario: Dashboard piped to another command

- **WHEN** the user runs `claude-usage | cat`
- **THEN** the output contains the dashboard text with no ANSI escape sequences

#### Scenario: Dashboard redirected to a file

- **WHEN** the user runs `claude-usage > out.txt`
- **THEN** `out.txt` contains no ANSI escape sequences

#### Scenario: Colors forced on a pipe

- **WHEN** the user runs `CLICOLOR_FORCE=1 claude-usage | cat`
- **THEN** the output contains color escape sequences

### Requirement: No colors when NO_COLOR is set

The dashboard SHALL write no color escape sequences when the `NO_COLOR` environment variable is set to any non-empty value, even on a terminal that supports color and even when `CLICOLOR_FORCE` is set.

#### Scenario: NO_COLOR on a color terminal

- **WHEN** the user runs `NO_COLOR=1 claude-usage` in a terminal that supports color
- **THEN** the output contains no foreground or background color escape sequences

#### Scenario: NO_COLOR with a non-numeric value

- **WHEN** the user runs `NO_COLOR=yes claude-usage` in a terminal that supports color
- **THEN** the output contains no foreground or background color escape sequences

#### Scenario: NO_COLOR and CLICOLOR_FORCE together

- **WHEN** the user runs `NO_COLOR=1 CLICOLOR_FORCE=1 claude-usage | cat`
- **THEN** the output contains no foreground or background color escape sequences

#### Scenario: Empty NO_COLOR

- **WHEN** the user runs `NO_COLOR= claude-usage` in a terminal that supports color
- **THEN** the output is colored as if `NO_COLOR` were not set

### Requirement: CLICOLOR and CLICOLOR_FORCE control colors

The dashboard SHALL write no color escape sequences when `CLICOLOR=0` is set, unless `CLICOLOR_FORCE` is set to a non-empty value other than `0`. Any non-empty `CLICOLOR_FORCE` value other than `0` SHALL force colors even when standard output is not a terminal. An empty `CLICOLOR_FORCE` or `CLICOLOR_FORCE=0` SHALL have no effect. `NO_COLOR` takes precedence over both variables.

#### Scenario: CLICOLOR=0 on a color terminal

- **WHEN** the user runs `CLICOLOR=0 claude-usage` in a terminal that supports color
- **THEN** the output contains no foreground or background color escape sequences

#### Scenario: CLICOLOR_FORCE overrides CLICOLOR=0

- **WHEN** the user runs `CLICOLOR=0 CLICOLOR_FORCE=1 claude-usage` in a terminal that supports color
- **THEN** the output is colored

#### Scenario: CLICOLOR_FORCE with a non-numeric value

- **WHEN** the user runs `CLICOLOR_FORCE=yes claude-usage | cat`
- **THEN** the output contains color escape sequences

#### Scenario: CLICOLOR_FORCE=0 on a pipe

- **WHEN** the user runs `CLICOLOR_FORCE=0 claude-usage | cat`
- **THEN** the output contains no ANSI escape sequences

#### Scenario: NO_COLOR overrides CLICOLOR_FORCE

- **WHEN** the user runs `NO_COLOR=1 CLICOLOR_FORCE=2 claude-usage | cat`
- **THEN** the output contains no color escape sequences

### Requirement: Colors on a color terminal

The dashboard SHALL render its colors when standard output is a terminal that supports color and `NO_COLOR` is not set, downgrading them to the color depth the terminal supports.

#### Scenario: 256-color terminal

- **WHEN** the user runs `claude-usage` in a terminal that supports 256 colors and `NO_COLOR` is not set
- **THEN** titles, utilization bars, and table headers are colored

#### Scenario: 16-color terminal

- **WHEN** the user runs `claude-usage` in a terminal that supports only 16 colors
- **THEN** the output uses only 16-color escape sequences
