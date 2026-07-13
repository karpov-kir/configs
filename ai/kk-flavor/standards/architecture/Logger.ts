/**
 * Log levels (lower number = more verbose). `Silent` is highest, so it disables all output (e.g. in tests).
 */
export enum LogLevel {
  Debug = 10,
  Info = 20,
  Warn = 30,
  Error = 40,
  Silent = 100,
}

interface LoggerOptions {
  logLevel?: LogLevel;
  stringifyObjects?: boolean;
  prefix?: string;
}

const isErrorLike = (object: unknown): object is { name: unknown; message: unknown; stack?: unknown } => {
  return (
    typeof object === 'object' &&
    object !== null &&
    'name' in object &&
    'message' in object &&
    typeof object.name === 'string' &&
    typeof object.message === 'string'
  );
};

/**
 * Level-based logging with optional scope labels.
 *
 * Use `Logger.root.scope('Label')` to create a child logger that automatically prepends
 * `[Label]` to every message. Scopes can be nested:
 *
 *   const log = Logger.root.scope('Editor').scope('LOD');
 *   log.debug('foo'); // → [CS ...] [DEBUG] [Editor] [LOD] foo
 *
 * Level and stringify are app-global — one root drives the whole app — so `setLevel(...)`
 * on any scope affects every logger.
 */
export class Logger {
  private static logLevel: LogLevel = LogLevel.Silent;
  private static stringifyObjects = false;

  private readonly prefix: string;

  private constructor({ logLevel, stringifyObjects, prefix = '' }: LoggerOptions = {}) {
    this.prefix = prefix;
    if (logLevel !== undefined) {
      Logger.logLevel = logLevel;
    }
    if (stringifyObjects !== undefined) {
      Logger.stringifyObjects = stringifyObjects;
    }
  }

  /**
   * The single app-wide logger. The constructor is private so no other root can be built — every log
   * traces back to this instance. Derive labeled children with `scope()`.
   */
  static readonly root = new Logger({ logLevel: LogLevel.Info, stringifyObjects: true });

  /**
   * Create a scoped child logger that prepends `[label]` to all messages.
   */
  scope(label: string): Logger {
    const prefix = this.prefix ? `${this.prefix} [${label}]` : `[${label}]`;
    return new Logger({ prefix });
  }

  private shouldLog(level: LogLevel): boolean {
    return level >= Logger.logLevel;
  }

  private stringifyArg(arg: unknown): unknown {
    if (!Logger.stringifyObjects) {
      return arg;
    }

    if (arg === null || arg === undefined) {
      return arg;
    }

    if (isErrorLike(arg)) {
      return JSON.stringify({
        name: arg.name,
        message: arg.message,
        stack: arg.stack,
      });
    }

    if (typeof arg === 'object') {
      try {
        return JSON.stringify(
          arg,
          (_key, value) =>
            isErrorLike(value) ? { name: value.name, message: value.message, stack: value.stack } : value,
          2,
        );
      } catch {
        return String(arg);
      }
    }

    return arg;
  }

  private formatArgs({ level, args }: { level: string; args: unknown[] }): unknown[] {
    const timestamp = new Date().toISOString().split('T')[1].split('Z')[0];
    const prefixPart = this.prefix ? ` ${this.prefix}` : '';
    const processedArgs = args.map((arg) => this.stringifyArg(arg));
    return [`[CS ${timestamp}] [${level}]${prefixPart}`, ...processedArgs];
  }

  /**
   * Debug-level logging (most verbose)
   */
  debug(...args: unknown[]): void {
    if (this.shouldLog(LogLevel.Debug)) {
      console.log(...this.formatArgs({ level: 'DEBUG', args }));
    }
  }

  /**
   * Info-level logging (default)
   */
  info(...args: unknown[]): void {
    if (this.shouldLog(LogLevel.Info)) {
      console.log(...this.formatArgs({ level: 'INFO', args }));
    }
  }

  /**
   * Warning-level logging
   */
  warn(...args: unknown[]): void {
    if (this.shouldLog(LogLevel.Warn)) {
      console.warn(...this.formatArgs({ level: 'WARN', args }));
    }
  }

  /**
   * Error-level logging
   */
  error(...args: unknown[]): void {
    if (this.shouldLog(LogLevel.Error)) {
      console.error(...this.formatArgs({ level: 'ERROR', args }));
    }
  }

  /**
   * Start a collapsible log group
   */
  group(label: string): void {
    if (this.shouldLog(LogLevel.Info)) {
      console.group(label);
    }
  }

  /**
   * Start a collapsed log group
   */
  groupCollapsed(label: string): void {
    if (this.shouldLog(LogLevel.Info)) {
      console.groupCollapsed(label);
    }
  }

  /**
   * End the current log group
   */
  groupEnd(): void {
    if (this.shouldLog(LogLevel.Info)) {
      console.groupEnd();
    }
  }

  /**
   * Set the log level. Use `LogLevel.Silent` to disable all output (e.g. in tests).
   */
  setLevel(level: LogLevel): void {
    Logger.logLevel = level;
  }
}
