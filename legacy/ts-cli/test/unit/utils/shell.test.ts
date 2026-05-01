import { describe, expect, test } from 'bun:test'
import { assertSafeName, formatEnvLine, shellEscape } from '../../../src/utils/shell.ts'

describe('shellEscape', () => {
	test('wraps simple string in single quotes', () => {
		expect(shellEscape('hello')).toBe("'hello'")
	})

	test('escapes embedded single quotes', () => {
		expect(shellEscape("it's")).toBe("'it'\\''s'")
	})

	test('handles empty string', () => {
		expect(shellEscape('')).toBe("''")
	})

	test('handles string with spaces', () => {
		expect(shellEscape('hello world')).toBe("'hello world'")
	})

	test('handles string with special shell characters', () => {
		expect(shellEscape('$(whoami)')).toBe("'$(whoami)'")
	})

	test('handles string with backticks', () => {
		expect(shellEscape('`rm -rf /`')).toBe("'`rm -rf /`'")
	})

	test('handles string with double quotes', () => {
		expect(shellEscape('"quoted"')).toBe('\'"quoted"\'')
	})

	test('handles string with newlines', () => {
		expect(shellEscape('line1\nline2')).toBe("'line1\nline2'")
	})

	test('handles multiple single quotes', () => {
		expect(shellEscape("a'b'c")).toBe("'a'\\''b'\\''c'")
	})
})

describe('formatEnvLine', () => {
	test('formats simple key=value', () => {
		expect(formatEnvLine('KEY', 'value')).toBe('KEY="value"')
	})

	test('escapes double quotes in value', () => {
		expect(formatEnvLine('KEY', 'val"ue')).toBe('KEY="val\\"ue"')
	})

	test('escapes dollar signs', () => {
		expect(formatEnvLine('KEY', '$HOME')).toBe('KEY="\\$HOME"')
	})

	test('escapes backticks', () => {
		expect(formatEnvLine('KEY', '`cmd`')).toBe('KEY="\\`cmd\\`"')
	})

	test('escapes backslashes', () => {
		expect(formatEnvLine('KEY', 'a\\b')).toBe('KEY="a\\\\b"')
	})

	test('escapes newlines', () => {
		expect(formatEnvLine('KEY', 'line1\nline2')).toBe('KEY="line1\\nline2"')
	})

	test('handles empty value', () => {
		expect(formatEnvLine('KEY', '')).toBe('KEY=""')
	})
})

describe('assertSafeName', () => {
	test('accepts valid lowercase names', () => {
		expect(() => assertSafeName('myapp', 'app')).not.toThrow()
		expect(() => assertSafeName('my-app', 'app')).not.toThrow()
		expect(() => assertSafeName('my_app', 'app')).not.toThrow()
		expect(() => assertSafeName('app123', 'app')).not.toThrow()
	})

	test('rejects names starting with a digit', () => {
		expect(() => assertSafeName('1app', 'app')).toThrow()
	})

	test('rejects names starting with a hyphen', () => {
		expect(() => assertSafeName('-app', 'app')).toThrow()
	})

	test('rejects uppercase letters', () => {
		expect(() => assertSafeName('MyApp', 'app')).toThrow()
	})

	test('rejects spaces', () => {
		expect(() => assertSafeName('my app', 'app')).toThrow()
	})

	test('rejects empty string', () => {
		expect(() => assertSafeName('', 'app')).toThrow()
	})

	test('includes label in error message', () => {
		expect(() => assertSafeName('BAD!', 'container')).toThrow(/container/)
	})
})
