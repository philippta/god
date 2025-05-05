## God, as in (Go) (D)ebugger

... maybe I'll think of a better name in the future.

### Why

- Delve is a bit clunky to use and I don't like vscode or golan*d*.
- gdb-dashboard looks cool and gets the job done.

### Screenshot

![Screenshot](/.github/screenshot1.png)

### Commands

| Command            | Alias              | Description                                                                                                                          |
| :----------------- | :----------------- | :----------------------------------------------------------------------------------------------------------------------------------- |
| `continue`         | `c`                | Continue program execution until the next breakpoint or program termination.                                                       |
| `next`             | `n`                | Step to the next source line in the current function, stepping *over* function calls.                                               |
| `step`             | `s`                | Step to the next source line, stepping *into* function calls.                                                                      |
| `stepout`          | `so`               | Continue execution until the current function returns.                                                                              |
| `nexti`            | `ni`               | Step to the next CPU instruction, stepping *over* function calls.                                                                   |
| `stepi`            | `si`               | Step to the next CPU instruction, stepping *into* function calls.                                                                   |
| `quit`             | `q`                | Exit the debugger session.                                                                                                         |
| `pane source`      | `pane src`         | Toggle the visibility of the source code pane.                                                                                     |
| `pane assembly`    | `pane asm`         | Toggle the visibility of the assembly code pane.                                                                                   |
| `pane variables`   | `pane vars`        | Toggle the visibility of the variables pane.                                                                                       |
| `pane breakpoints` | `pane bp`          | Toggle the visibility of the breakpoints pane.                                                                                     |
| `pane watch`       |                    | Toggle the visibility of the watch pane.                                                                                           |
| `grow source`      | `grow src`         | Increase the height of the source code pane by 2 lines.                                                                             |
| `grow assembly`    | `grow asm`         | Increase the height of the assembly code pane by 2 lines.                                                                           |
| `shrink source`    | `shrink src`       | Decrease the height of the source code pane by 2 lines (minimum height is 1).                                                      |
| `shrink assembly`  | `shrink asm`       | Decrease the height of the assembly code pane by 2 lines (minimum height is 1).                                                      |
| `break <location>` | `b <location>`     | Create a breakpoint at the specified location. `<location>` can be `file:line`, `line` (in the current file), or a `functionName`. |
| `clear <id>`       | `c <id>`           | Remove (clear) the breakpoint with the specified numeric ID.                                                                       |
| `watch <expr>`     | `w <expr>`         | Add the specified expression `<expr>` to the watch list.                                                                             |
| `unwatch <arg>`    | `uw <arg>`         | Remove a watch entry. `<arg>` can be the numeric ID (starting from 1) or the exact expression string.                             |
| `(empty command)`  |                    | Repeat the last command that was entered.                                                                                          |

**Notes:**

* `<id>` refers to the numeric identifier of a breakpoint (for `clear`) or a watch entry (for `unwatch`).
* `<location>` specifies where to set a breakpoint (`file:line`, `line`, `functionName`).
* `<expr>` refers to the variable or expression string to be watched.
* `<arg>` for `unwatch` can be either a numeric ID or the expression string itself.
* The alias `c` is used for `continue`, while `c <id>` is used for `clear <id>`. The space and argument differentiate them. Similarly, `w <expr>` and `uw <arg>` require arguments.


### Disclaimer

This is early stage, just a couple hours in. Pls don't read the code. Use at your own risk.
