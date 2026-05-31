export const DEFAULT_HTTP_ENDPOINT: "http://127.0.0.1:7423";
export const REDACTED_VALUE: "[REDACTED]";

export type Level = "DEBUG" | "INFO" | "WARN" | "ERROR" | string;
export type RecordAttrs = Record<string, unknown>;

export interface DevLogBusRecord {
  time?: string | Date;
  level?: Level;
  source?: string;
  message: string;
  attrs?: RecordAttrs;
}

export interface PreparedDevLogBusRecord {
  time: string;
  level: string;
  source: string;
  message: string;
  attrs?: RecordAttrs;
}

export type RecordFilter = (record: PreparedDevLogBusRecord) => boolean;
export type RecordRedactor = (record: PreparedDevLogBusRecord) => PreparedDevLogBusRecord;
export type FetchLike = (
  url: string,
  init: {
    method: string;
    headers: Record<string, string>;
    body: string;
  },
) => Promise<{
  ok: boolean;
  status: number;
  json(): Promise<PublishResult>;
}>;

export interface DevLogBusClientOptions {
  endpoint?: string;
  source?: string;
  fetch?: FetchLike;
  filter?: RecordFilter;
  redactor?: RecordRedactor;
}

export interface PublishResult {
  published: number;
  filtered?: boolean;
}

export interface DevLogBusLogger {
  debug(message: string, attrs?: RecordAttrs): Promise<PublishResult>;
  info(message: string, attrs?: RecordAttrs): Promise<PublishResult>;
  warn(message: string, attrs?: RecordAttrs): Promise<PublishResult>;
  error(message: string, attrs?: RecordAttrs): Promise<PublishResult>;
}

export declare function normalizeLevel(level?: Level): string;
export declare function createRecord(input: DevLogBusRecord, defaultSource?: string): PreparedDevLogBusRecord;
export declare function createLogger(options?: DevLogBusClientOptions): DevLogBusLogger;
export declare function dropSources(sources?: string[]): RecordFilter;
export declare function redactAttrs(keys?: string[], replacement?: unknown): RecordRedactor;

export declare class DevLogBusClient {
  constructor(options?: DevLogBusClientOptions);
  publish(input: DevLogBusRecord): Promise<PublishResult>;
  publishBatch(records: DevLogBusRecord[]): Promise<PublishResult>;
  logger(source?: string): DevLogBusLogger;
}
