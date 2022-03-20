CREATE MATERIALIZED VIEW samplers_outif TO samplers
AS SELECT
    TimeReceived,
    SamplerAddress,
    SamplerName,
    SamplerGroup,
    OutIfName AS IfName,
    OutIfDescription AS IfDescription,
    OutIfSpeed AS IfSpeed,
    OutIfConnectivity AS IfConnectivity,
    OutIfProvider AS IfProvider,
    OutIfBoundary AS IfBoundary
   FROM flows
