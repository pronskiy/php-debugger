# PHP Debugger CLI

A command-line debugger for PHP using the DBGp protocol. Works with the `php_debugger` extension.

## Build

```bash
cd cli && make build
```

Binary: `bin/dbg`

## Usage

```
PHP Debugger CLI - DBGp Protocol Debug Tool

DESCRIPTION
  A command-line debugger for PHP applications using the DBGp protocol.
  Listens for incoming connections from PHP's Xdebug or php-debugger extension
  to set breakpoints, inspect variables, and step through code execution.

USAGE
  dbg [options]

OPTIONS
  -port <int>           TCP port for DBGp connection (default: 9003)
  -timeout <duration>   Time to wait for PHP connection before exiting (default: 30s)
  -breakpoint <spec>    Set breakpoint, can be repeated (see format below)
  -raw                  Output raw XML responses below parsed variables

BREAKPOINT FORMAT
  file.php:42              Single breakpoint at line 42
  file.php:10,20,30        Multiple lines in same file
  path/to/file.php:100     With relative or absolute path

EXAMPLES
  dbg -breakpoint app.php:42
  dbg -breakpoint Controller.php:10,20,30 -breakpoint Model.php:50
  dbg -port 9003

RUNNING PHP SCRIPTS
  dbg -breakpoint app.php:42 app.php
  dbg -breakpoint Controller.php:10 app.php [script args...]
```

## Example with Symfony

Terminal 1 - Start the debugger:
```bash
dbg -breakpoint src/Controller/HomeController.php:25
```

Terminal 2 - Start Symfony with debugging enabled:
```bash
PHP_DEBUGGER_TRIGGER=1 symfony server:start
```

When your app hits the breakpoint, the debugger shows variables and stack trace.

```
Listening on 127.0.0.1:9003
Waiting for PHP to connect...

✓ Connected: /home/daniel/projects/espend-de/public/index.php
✓ Breakpoint: HomeController.php:36

→ Running...

━━━ HomeController.php:36 ━━━

Stack:
#0 App\Controller\HomeController->index() at HomeController.php:36
#1 Symfony\Component\HttpKernel\HttpKernel->handleRaw() at HttpKernel.php:183
#2 Symfony\Component\HttpKernel\HttpKernel->handle() at HttpKernel.php:76
#3 Symfony\Component\HttpKernel\Kernel->handle() at Kernel.php:193
#4 Symfony\Component\Runtime\Runner\Symfony\HttpKernelRunner->run() at HttpKernelRunner.php:35
#5 require_once() at autoload_runtime.php:32
#6 {main}() at index.php:5

Variables:
$featureCollector              object   = App\Service\SymfonyFeatureCollector
  $featureCollector->projectDir string   = "/home/daniel/projects/espend-de"
  $featureCollector->cache     object   = Symfony\Component\Cache\Adapter\TraceableAdapter
$github                        uninitialized = null
$latestFeatures                array    = array[14]
  $latestFeatures[0]           array    = array[3]
  $latestFeatures[1]           array    = array[3]
  $latestFeatures[2]           array    = array[3]
  $latestFeatures[3]           array    = array[3]
  $latestFeatures[4]           array    = array[3]
  $latestFeatures[5]           array    = array[3]
  $latestFeatures[6]           array    = array[3]
  $latestFeatures[7]           array    = array[3]
  $latestFeatures[8]           array    = array[3]
  $latestFeatures[9]           array    = array[3]
  $latestFeatures[10]          array    = array[3]
  $latestFeatures[11]          array    = array[3]
  $latestFeatures[12]          array    = array[3]
  $latestFeatures[13]          array    = array[3]
$p                             null     = null
$plugin                        uninitialized = null
$pluginRepository              object   = App\Repository\PluginRepository{}
$pluginUpdateInfo              array    = array[6]
  $pluginUpdateInfo["symfony"] array    = array[4]
  $pluginUpdateInfo["shopware"] array    = array[4]
  $pluginUpdateInfo["php-annotations"] array    = array[4]
  $pluginUpdateInfo["laravel"] array    = array[4]
  $pluginUpdateInfo["phpunit"] array    = array[4]
  $pluginUpdateInfo["vuejs-toolbox"] array    = array[4]
```