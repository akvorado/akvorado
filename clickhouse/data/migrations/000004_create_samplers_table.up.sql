CREATE TABLE samplers
(
    TimeReceived DateTime,
    SamplerAddress LowCardinality(IPv6),
    SamplerName LowCardinality(String),
    SamplerGroup LowCardinality(String),

    IfName LowCardinality(String),
    IfDescription String,
    IfSpeed UInt32,
    IfConnectivity LowCardinality(String),
    IfProvider LowCardinality(String),
    IfBoundary Enum('undefined' = 0, 'external' = 1, 'internal' = 2)
)
ENGINE = ReplacingMergeTree(TimeReceived)
ORDER BY (SamplerAddress, IfName)
