package com.dansherwin.devlogbus;

@FunctionalInterface
public interface RecordRedactor {
    DevLogBusRecord redact(DevLogBusRecord record);
}
