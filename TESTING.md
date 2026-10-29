# Testing Documentation

This document describes the testing strategy and structure for the GPG Password Store Viewer application.

## Test Structure

The application follows Go testing best practices with comprehensive unit tests, integration tests, and benchmarks.

### Test Files

- `main_test.go` - Tests for main application logic
- `scanpassstore/scan_test.go` - Tests for password store scanning functionality
- `settings/settings_test.go` - Tests for application settings management
- `settings/theme_test.go` - Tests for theme handling

## Test Coverage

### Main Package (`main_test.go`)
- **TestDecryptCommandConstruction**: Tests GPG command construction logic
- **TestScanPasswordStoreCLI**: Tests CLI scanning functionality with temp directories

**Coverage**: 0.0% (main.go contains mostly GUI logic which is not unit tested)

### ScanPassStore Package (`scanpassstore/scan_test.go`)
- **TestScanPasswordStore**: Tests scanning of complex directory structures
- **TestScanPasswordStoreEmptyDirectory**: Tests handling of empty directories
- **TestScanPasswordStoreNonExistentDirectory**: Tests error handling for non-existent paths
- **TestScanPasswordStoreWithNonGpgFiles**: Tests filtering of non-GPG files
- **TestFindFilePath**: Tests recursive file path finding
- **TestPasswordStoreStructure**: Tests PasswordStore struct creation and validation
- **BenchmarkScanPasswordStore**: Performance benchmark for scanning large directory structures

**Coverage**: 87.8% of statements

### Settings Package (`settings/settings_test.go`)
- **TestDefaultSettings**: Tests default settings creation
- **TestLoadSettingsNewFile**: Tests loading settings when file doesn't exist
- **TestLoadSettingsExistingFile**: Tests loading existing settings
- **TestSaveSettings**: Tests saving settings to file
- **TestUpdateSettings**: Tests updating specific settings
- **TestUpdateSettingsInvalidKey**: Tests handling of invalid setting keys
- **TestUpdateSettingsInvalidType**: Tests handling of invalid setting types
- **TestLoadSettingsCorruptedFile**: Tests handling of corrupted settings files
- **TestSaveSettingsPermissionError**: Tests directory creation for settings
- **BenchmarkLoadSettings**: Performance benchmark for loading settings
- **BenchmarkSaveSettings**: Performance benchmark for saving settings

**Coverage**: 49.1% of statements

### Theme Package (`settings/theme_test.go`)
- **TestApplyTheme**: Tests theme application with mocked Fyne app
- **TestGetAvailableThemes**: Tests available themes list
- **TestGetThemeDisplayName**: Tests theme display name mapping

## Running Tests

### Basic Test Commands

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Run tests with coverage
go test ./... -cover

# Run benchmarks
go test ./... -bench=. -benchmem
```

### Using Makefile

```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Run tests with coverage report
make test-coverage

# Run benchmarks
make test-benchmark
```

### Package-Specific Tests

```bash
# Test only scanpassstore package
go test ./scanpassstore

# Test only settings package
go test ./settings

# Test only main package
go test ./main.go
```

## Test Environment

### Temporary Directories
All tests use temporary directories created with `os.MkdirTemp()` to ensure isolation and cleanup.

### Environment Variables
Settings tests override the `HOME` environment variable to use temporary directories for configuration files.

### Mocking
- Fyne app and settings are mocked for theme testing
- File system operations are tested with real temporary files
- GPG commands are tested for construction, not execution

## Test Data

### Directory Structures
Tests create various directory structures to verify:
- Nested directories with .gpg files
- Empty directories
- Mixed file types
- Non-existent paths

### Settings Data
Tests verify:
- Default settings values
- JSON serialization/deserialization
- Invalid data handling
- File permission scenarios

## Performance Benchmarks

### ScanPassStore Benchmarks
- **BenchmarkScanPasswordStore**: Tests scanning 100 directories with 10 files each
- Average: ~1ms per operation with ~317KB memory allocation

### Settings Benchmarks
- **BenchmarkLoadSettings**: Tests loading settings from file
- **BenchmarkSaveSettings**: Tests saving settings to file
- Both operations are optimized for typical usage patterns

## Best Practices Followed

1. **Isolation**: Each test uses temporary directories and files
2. **Cleanup**: All temporary resources are cleaned up with `defer`
3. **Assertions**: Using `testify` for clear, descriptive assertions
4. **Error Handling**: Testing both success and error scenarios
5. **Mocking**: Minimal mocking, focusing on real behavior where possible
6. **Benchmarks**: Performance testing for critical operations
7. **Coverage**: Aiming for high test coverage of business logic

## Continuous Integration

Tests are designed to run in CI environments:
- No external dependencies (GPG, GUI)
- Fast execution (< 30 seconds total)
- Deterministic results
- Clear error messages

## Future Improvements

1. **Integration Tests**: Add tests for actual GPG operations (requires test keys)
2. **GUI Tests**: Add tests for Fyne UI components (requires headless environment)
3. **End-to-End Tests**: Add tests for complete user workflows
4. **Performance Tests**: Add more comprehensive benchmarking
5. **Security Tests**: Add tests for encryption/decryption edge cases

## Troubleshooting

### Common Issues

1. **Permission Errors**: Tests use temporary directories to avoid permission issues
2. **Race Conditions**: Tests are designed to be independent and non-racing
3. **Memory Leaks**: All resources are properly cleaned up
4. **Slow Tests**: Benchmarks are run separately from unit tests

### Debug Mode

To run tests with additional debugging:

```bash
# Run with race detection
go test -race ./...

# Run with verbose output and coverage
go test -v -cover ./...

# Run specific test with debugging
go test -v -run TestSpecificTest ./...
``` 