CREATE TABLE flows
(
    Date Date,
    TimeReceived DateTime,

    SequenceNum UInt32,
    SamplingRate UInt64,
    SamplerAddress LowCardinality(IPv6),
    SamplerName LowCardinality(String),
    SamplerGroup LowCardinality(String),

    SrcAddr IPv6,
    DstAddr IPv6,

    SrcAS UInt32,
    DstAS UInt32,
    SrcCountry FixedString(2),
    DstCountry FixedString(2),

    InIfName LowCardinality(String),
    OutIfName LowCardinality(String),
    InIfDescription String,
    OutIfDescription String,
    InIfSpeed UInt32,
    OutIfSpeed UInt32,
    InIfConnectivity LowCardinality(String),
    OutIfConnectivity LowCardinality(String),
    InIfProvider LowCardinality(String),
    OutIfProvider LowCardinality(String),
    InIfBoundary Enum('undefined' = 0, 'external' = 1, 'internal' = 2),
    OutIfBoundary Enum('undefined' = 0, 'external' = 1, 'internal' = 2),

    EType UInt32,
    Proto UInt32,

    SrcPort UInt32,
    DstPort UInt32,

    Bytes UInt64,
    Packets UInt64
) ENGINE = MergeTree()
PARTITION BY Date
ORDER BY TimeReceived
