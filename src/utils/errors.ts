export class ShuttleError extends Error {
	readonly code: string

	constructor(message: string, code: string) {
		super(message)
		this.name = this.constructor.name
		this.code = code
		// Restore prototype chain (required when extending built-ins in TS)
		Object.setPrototypeOf(this, new.target.prototype)
	}

	static wrap(err: unknown, message?: string): ShuttleError {
		if (err instanceof ShuttleError) return err
		const cause = err instanceof Error ? err.message : String(err)
		return new ShuttleError(message ? `${message}: ${cause}` : cause, 'SHUTTLE_ERROR')
	}
}

export class ConfigError extends ShuttleError {
	constructor(message: string) {
		super(message, 'CONFIG_ERROR')
	}

	static wrap(err: unknown, message?: string): ConfigError {
		if (err instanceof ConfigError) return err
		const cause = err instanceof Error ? err.message : String(err)
		return new ConfigError(message ? `${message}: ${cause}` : cause)
	}
}

export class SSHError extends ShuttleError {
	readonly host?: string

	constructor(message: string, host?: string) {
		super(message, 'SSH_ERROR')
		this.host = host
	}

	static wrap(err: unknown, message?: string, host?: string): SSHError {
		if (err instanceof SSHError) return err
		const cause = err instanceof Error ? err.message : String(err)
		return new SSHError(message ? `${message}: ${cause}` : cause, host)
	}
}

export class DeployError extends ShuttleError {
	readonly phase?: string

	constructor(message: string, phase?: string) {
		super(message, 'DEPLOY_ERROR')
		this.phase = phase
	}

	static wrap(err: unknown, message?: string, phase?: string): DeployError {
		if (err instanceof DeployError) return err
		const cause = err instanceof Error ? err.message : String(err)
		return new DeployError(message ? `${message}: ${cause}` : cause, phase)
	}
}

export class SecretsError extends ShuttleError {
	constructor(message: string) {
		super(message, 'SECRETS_ERROR')
	}

	static wrap(err: unknown, message?: string): SecretsError {
		if (err instanceof SecretsError) return err
		const cause = err instanceof Error ? err.message : String(err)
		return new SecretsError(message ? `${message}: ${cause}` : cause)
	}
}

export class ProvisionError extends ShuttleError {
	constructor(message: string) {
		super(message, 'PROVISION_ERROR')
	}

	static wrap(err: unknown, message?: string): ProvisionError {
		if (err instanceof ProvisionError) return err
		const cause = err instanceof Error ? err.message : String(err)
		return new ProvisionError(message ? `${message}: ${cause}` : cause)
	}
}
