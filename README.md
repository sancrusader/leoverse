# Leoverse

Leoverse is a Go library and command-line tool for generating images using Leonardo AI's services.

## Installation

```bash
go get github.com/sancrusader/leoverse
```

## Configuration

Before using Leoverse, you need to set up your Leonardo AI credentials. The application looks for a cookie file in the following location:

```
cmd/leonai/cookie.txt
```

Format your cookie file with your Leonardo AI credentials:

```
cookie.txt
```

## Usage

### Command Line Interface

To generate an image using the CLI:

```bash
./leoverse generate --prompt "your creative prompt here"
```

### Programmatic Usage

```go
package main

import (
    "github.com/sancrusader/leoverse"
    "github.com/sancrusader/leoverse/pkg/leonardo"
)

func main() {
    client := leonardo.NewClient("your_cookie_value")

    // Generate an image
    response, err := client.GenerateImage("your creative prompt")
    if err != nil {
        panic(err)
    }

    // Handle the response
    // ...
}
```

## Project Structure

```
├── cmd/
│   └── leoverse/        # CLI implementation
├── pkg/
│   ├── leonardo/      # Leonardo AI API client
│   ├── ratelimit/     # Rate limiting functionality
│   └── session/       # Session management
└── generate_image.go  # Main image generation logic
```

## Features

- Image generation using Leonardo AI
- Rate limiting support
- Session management
- Easy-to-use CLI interface
- Programmatic API access

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
