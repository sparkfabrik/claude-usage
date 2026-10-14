# Spec Delta

## Purpose

Defines how the terminal dashboard adapts its colors and text styling to where its output goes, so it reads well in a terminal and stays clean when piped or when the user opts out of colors.

## ADDED Requirements

### Requirement: Plain output when not writing to a terminal
The dashboard SHALL write no ANSI escape sequences when standard output is not a terminal, such as a pipe or a file.

#### Scenario: Dashboard piped to another command
- **WHEN** the user runs `claude-usage | cat`
- **THEN** the output contains the dashboard text with no ANSI escape sequences

#### Scenario: Dashboard redirected to a file
- **WHEN** the user runs `claude-usage > out.txt`
- **THEN** `out.txt` contains no ANSI escape sequences

### Requirement: No colors when NO_COLOR is set
The dashboard SHALL write no color escape sequences when the `NO_COLOR` environment variable is set to a non-empty value, even on a terminal that supports color.

#### Scenario: NO_COLOR on a color terminal
- **WHEN** the user runs `NO_COLOR=1 claude-usage` in a terminal that supports color
- **THEN** the output contains no foreground or background color escape sequences

### Requirement: Colors on a color terminal
The dashboard SHALL render its colors when standard output is a terminal that supports color and `NO_COLOR` is not set, downgrading them to the color depth the terminal supports.

#### Scenario: 256-color terminal
- **WHEN** the user runs `claude-usage` in a terminal that supports 256 colors and `NO_COLOR` is not set
- **THEN** titles, utilization bars, and table headers are colored

#### Scenario: 16-color terminal
- **WHEN** the user runs `claude-usage` in a terminal that supports only 16 colors
- **THEN** the output uses only 16-color escape sequences
