CREATE MATERIALIZED VIEW samplers_inif TO samplers
AS SELECT
    TimeReceived,
    SamplerAddress,
    SamplerName,
    SamplerGroup,
    InIfName AS IfName,
    InIfDescription AS IfDescription,
    InIfSpeed AS IfSpeed,
    InIfConnectivity AS IfConnectivity,
    InIfProvider AS IfProvider,
    InIfBoundary AS IfBoundary
   FROM flows
