# PHP Debugger

A PHP debugger extension focused on step debugging with near-zero overhead. Forked from [Xdebug](https://xdebug.org/), with profiling, coverage, and tracing removed.

> [!NOTE]
> **🧪 This project is an experiment** exploring minimal-overhead PHP debugging.

## Why PHP Debugger?

- **Near-zero overhead** when loaded but no debug client is connected
- **Xdebug-compatible** — existing configs, IDE setups, and workflows work unchanged
- **Debug-only** — focused exclusively on step debugging
- **Full DBGp protocol support** — works with PhpStorm, VS Code, and any DBGp-compatible IDE

### Benchmarks

The following benchmarks were run in GitHub's CI (GitHub Actions) environment using a standard Ubuntu runner. 
The performance was measured using Valgrind to count the number of executed instructions.
This is much more precise and reproducible than timing execution runs. All measuremments were done
using all supported PHP versions (the number shown is the average), with the extension loaded and
no IDE connected.

We measured three different scenarios which we believe represent a good mix of typical PHP operations:

- `bench.php`: a syntetic benchmark that runs a number of computationally heavy functions

| Configuration |    Overhead |
|---------------|------------:|
| No debugger   |           — |
| Xdebug        | **+661.6%** |
| PHP Debugger  |  **+12.9%** |

- `Rector`: running a RectorPHP rule on a PHP file

| Configuration |    Overhead |
|---------------|------------:|
| No debugger   |           — |
| Xdebug        | **+124.5%** |
| PHP Debugger  |   **+3.6%** |

- `Symfony`: running a basic request on a Symfony demo project

| Configuration |   Overhead |
|---------------|-----------:|
| No debugger   |          — |
| Xdebug        | **+35.3%** |
| PHP Debugger  |  **+1.3%** |

### On-Demand Debugging

To improve the performance of code running with the PHP Debugger enabled, several features required for on-demand
debugging are disabled if the debugger does not connect to a client at startup.

On-demand debugging allows the debugger to connect later during execution—for example, via `xdebug_connect_to_client()`,
`xdebug_break()`, or when an error or exception occurs.

We consider on-demand debugging to be a relatively uncommon use case, and we want to avoid degrading performance for
the majority of users. However, since some users rely on this functionality, we provide an INI setting to enable
it when needed.

INI setting: `php_debugger.on_demand_debugging_enabled` (default: false)

When this setting is enabled, on-demand debugging features remain active even if no client is connected at startup. 
Note that this has a significant performance impact: instead of achieving up to a 97% performance improvement, the average improvement drops to around 60%.

On top of that, every request must be compiled with debugging instrumentation, so OPcache is bypassed for *all* requests in the
process — not just the ones that end up being debugged. On a busy server (PHP-FPM in particular) that recompilation is a
substantial throughput cost. The debugger logs a `[Config] INFO` line about this at request start, and `xdebug_info()` reports the
bypass in its Step Debugging section.

For this reason, we recommend enabling this setting only if you specifically require on-demand debugging.

## Installation

### Docker

Drop-in replacements for the [official PHP images](https://hub.docker.com/_/php) with PHP Debugger statically compiled in — just add the `phpdebugger/` prefix to your base image:

```dockerfile
FROM phpdebugger/php:8.4-fpm
```

All official variants are available (`cli`, `fpm`, `apache`, `zts`, and their Alpine equivalents) for PHP 8.2–8.5, on amd64 and arm64. Tags exist per minor version only (no `8.4.23`); each always contains the latest patch release. Everything from the official images works unchanged, including `docker-php-ext-install`. See [docker/README.md](docker/README.md) for tags and debugging setup.

### Manual download

Grab the right binary from [Releases](https://github.com/php-debugger/php-debugger/releases), copy it to your extension directory, and add to `php.ini`:

```ini
zend_extension=php_debugger.so
```

### 🚧 Coming soon 

**Quick install script:**

```bash
curl -fsSL https://raw.githubusercontent.com/php-debugger/php-debugger/main/install.php | php
```

**PIE (PHP Installer for Extensions):**

```bash
pie install php-debugger/php-debugger
```

## Configuration

PHP Debugger accepts both `php_debugger.*` and `xdebug.*` INI prefixes. Existing Xdebug configurations work as-is.

```ini
; Both of these work:
php_debugger.mode = debug
php_debugger.client_host = 127.0.0.1
php_debugger.client_port = 9003
php_debugger.start_with_request = trigger

; Xdebug-compatible (also works):
xdebug.mode = debug
xdebug.client_host = 127.0.0.1
xdebug.client_port = 9003
xdebug.start_with_request = trigger
```

### OPcache

Debugging needs every file compiled with debugging information, and OPcache is shared between requests, so PHP Debugger switches
OPcache off for the requests it instruments — those with a debugging client attached, and all requests when
`php_debugger.on_demand_debugging_enabled` (aka `xdebug.on_demand_debugging_enabled`) is on. Other requests keep OPcache exactly as you configured it.

Without this, a file first compiled by a non-debugged request stays cached without debugging information: breakpoints in it never
fire and stepping walks straight past its functions. It also keeps instrumented code from being cached and slowing down requests
that are not being debugged. As a side effect, JIT does not run for debugged requests either — it is part of OPcache, and it is
incompatible with debugging anyway.

`xdebug_info()` says `OPcache is bypassed for this request` when this applies, so you can tell this apart from OPcache being off
for some other reason.

### FrankenPHP worker mode

The near-zero overhead described above relies on deciding per request whether to compile with debugging information. FrankenPHP
worker mode cannot work that way: one worker serves many requests from code it compiled once, before it can know that a later
request will ask to be debugged. PHP Debugger therefore compiles everything with debugging information for the whole worker
process, and every request pays for the per-statement dispatch that comes with it — including requests with no debugging trigger.
The dispatch bails out immediately when no client is connected, so the cost is small, but it is not the "you don't pay for what
you don't use" behaviour you get on CLI and PHP-FPM.

This applies as soon as the extension is loaded with `mode=debug`, whether or not you ever attach an IDE. Setting the mode off
(`php_debugger.mode=off`, `xdebug.mode=off`, or the `XDEBUG_MODE=off` environment variable) skips the instrumentation entirely, so
a worker you are not planning to debug runs at full speed.

## IDE Setup

### PhpStorm

Works with existing PhpStorm debug configurations. No IDE changes needed.

[Configuring Debugger in PhpStorm](https://www.jetbrains.com/help/phpstorm/configuring-xdebug.html)

### VS Code

Works as-is. No changes needed.

### Agents

[Agents CLI](cli/README.md)

```
dbg -breakpoint src/Controller/HomeController.php:25
```

## Xdebug Compatibility

PHP Debugger maintains compatibility with Xdebug's debug mode:

| Feature                            | PHP Debugger                                                                 | Xdebug |
|------------------------------------|------------------------------------------------------------------------------|--------|
| `extension_loaded("xdebug")`       | ✅ true                                                                       | ✅ true |
| `extension_loaded("php_debugger")` | ✅ true                                                                       | ❌ false |
| `xdebug.*` INI settings            | ✅ works                                                                      | ✅ works |
| `xdebug_break()`                   | ✅ works                                                                      | ✅ works |
| `XDEBUG_SESSION` trigger           | ✅ works                                                                      | ✅ works |
| Step debugging (DBGp)              | ✅                                                                            | ✅      |
| On-demand debugging                | ✅ works if `on_demand_debugging_enabled`<br/>is set, does not work otherwise | ✅      |
| Code coverage                      | ❌ use pcov                                                                   | ✅      |
| Profiling                          | ❌ removed                                                                    | ✅      |
| Tracing                            | ❌ removed                                                                    | ✅      |

### New names (optional)

You can also use the new names — they work alongside the Xdebug ones:

- **INI:** `php_debugger.mode`, `php_debugger.client_host`, etc.
- **Functions:** `php_debugger_break()`, `php_debugger_info()`, `php_debugger_connect_to_client()`, `php_debugger_is_debugger_active()`, `php_debugger_notify()`
- **Triggers:** `PHP_DEBUGGER_SESSION`, `PHP_DEBUGGER_SESSION_START`, `PHP_DEBUGGER_TRIGGER`

## Requirements

- PHP 8.2, 8.3, 8.4, or 8.5

## License

Released under [The Xdebug License](LICENSE), version 1.03 (based on The PHP License).

This product includes Xdebug software, freely available from [https://xdebug.org/](https://xdebug.org/).

## Acknowledgments

PHP Debugger is built on the foundation of [Xdebug](https://xdebug.org/), created and maintained by **Derick Rethans** since 2002. His two decades of work on PHP debugging tools made this project possible. Thank you, Derick.
