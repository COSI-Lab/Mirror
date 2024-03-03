# logging

This module provides thread-safe logging.

![Screenshot](screenshot.png)

## Usage

```go
package main

import (
    "github.com/COSI-Lab/Mirror/logging"
)

func main() {
    logging.Info("Hello, world!")
    logging.Warn("Warning world didn't say hello back!")
    logging.Error("Error world is broken!")
}
```
