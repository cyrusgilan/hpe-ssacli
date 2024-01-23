# ssacli-exporter

This is a simple go prometheus exporter that exposes array information from `ssacli`.
Unfortunately it seems there are no pre-made ones available.

It can be built and run like any other go program, and exposes metrics on `:9101`.
It only makes sense to install on physical servers, alongside [ssacli](https://downloads.linux.hpe.com/SDR/project/mcp/)
