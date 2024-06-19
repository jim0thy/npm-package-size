# Project Name

## Description

Checks the unzipped size of all packages in an NPM org

## Installation

1. **Clone the repository:**

   ```sh
   git clone https://github.com/yourusername/projectname.git
   cd projectname
   ```

2. **Install dependencies:**

   ```sh
   go mod tidy
   ```

3. **Build the project:**

   ```sh
   go build -o projectname
   ```

## Usage

1. **Run the project:**

   ```sh
   ./projectname
   ```

2. **Example command:**

   ```sh
   ./projectname arg1 arg2
   ```

## Development

### Prerequisites

- Go 1.19 or higher

### Running Tests

To run tests, use the following command:

```sh
go test ./...
```

### Linting

Ensure your code follows best practices by running the linter:

1. **Install `golangci-lint`:**

   ```sh
   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.50.1
   ```

2. **Run the linter:**

   ```sh
   golangci-lint run
   ```