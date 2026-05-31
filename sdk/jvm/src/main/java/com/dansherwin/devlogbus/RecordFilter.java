package com.dansherwin.devlogbus;

@FunctionalInterface
public interface RecordFilter {
    boolean shouldPublish(DevLogBusRecord record);
}
