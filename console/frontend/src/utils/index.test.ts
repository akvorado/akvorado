// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it } from "vitest";
import { formatXps, compareFields } from "./index";

describe("formatXps", () => {
  it("formats small values without suffix", () => {
    expect(formatXps(0)).toBe("0.00");
    expect(formatXps(1)).toBe("1.00");
    expect(formatXps(999)).toBe("999.00");
  });

  it("preserves negative sign", () => {
    expect(formatXps(-1)).toBe("-1.00");
    expect(formatXps(-1000)).toBe("-1.00K");
    expect(formatXps(-1500)).toBe("-1.50K");
  });

  it("formats values with K suffix", () => {
    expect(formatXps(1000)).toBe("1.00K");
    expect(formatXps(1500)).toBe("1.50K");
    expect(formatXps(999999)).toBe("1000.00K");
  });

  it("formats values with M suffix", () => {
    expect(formatXps(1000000)).toBe("1.00M");
    expect(formatXps(1500000)).toBe("1.50M");
    expect(formatXps(999999999)).toBe("1000.00M");
  });

  it("formats values with G suffix", () => {
    expect(formatXps(1000000000)).toBe("1.00G");
    expect(formatXps(1500000000)).toBe("1.50G");
    expect(formatXps(999999999999)).toBe("1000.00G");
  });

  it("formats values with T suffix", () => {
    expect(formatXps(1000000000000)).toBe("1.00T");
    expect(formatXps(1500000000000)).toBe("1.50T");
    expect(formatXps(999999999999999)).toBe("1000.00T");
  });

  it("formats values with P suffix", () => {
    expect(formatXps(1000000000000000)).toBe("1.00P");
    expect(formatXps(1500000000000000)).toBe("1.50P");
  });

  it("caps at P suffix for very large values", () => {
    expect(formatXps(1000000000000000000)).toBe("1000.00P");
    // eslint-disable-next-line no-loss-of-precision
    expect(formatXps(9999999999999999999)).toBe("10000.00P");
  });

  it("rounds to 2 decimal places", () => {
    expect(formatXps(1234)).toBe("1.23K");
    expect(formatXps(1236)).toBe("1.24K");
    expect(formatXps(1234567)).toBe("1.23M");
  });
});

describe("compareFields", () => {
  it("compares fields by prefix priority", () => {
    expect(compareFields("TimeReceived", "Bytes")).toBeLessThan(0);
    expect(compareFields("Bytes", "Packets")).toBeLessThan(0);
    expect(compareFields("ExporterName", "SamplingRate")).toBeLessThan(0);
    expect(compareFields("SamplingRate", "SrcCountry")).toBeLessThan(0);
    expect(compareFields("SrcCountry", "InIfName")).toBeLessThan(0);
    expect(compareFields("InIfBoundary", "DstAS")).toBeLessThan(0);
    expect(compareFields("DstNetNask", "OutIfDescription")).toBeLessThan(0);
    expect(compareFields("OutIfName", "Proto")).toBeLessThan(0);
    expect(compareFields("OutIfName", "EType")).toBeLessThan(0);
  });

  it("compares fields with same prefix alphabetically", () => {
    expect(compareFields("DstAddr", "DstAS")).toBeLessThan(0);
    expect(compareFields("SrcAddr", "SrcAS")).toBeLessThan(0);
  });

  it("handles unknown prefixes with fallback priority", () => {
    expect(compareFields("UnkField1", "UnkField2")).toBeLessThan(0);
    expect(compareFields("UnkField2", "UnkField1")).toBeGreaterThan(0);
    expect(compareFields("UnkField", "TimeReceived")).toBeGreaterThan(0);
    expect(compareFields("TimeReceived", "UnkField")).toBeLessThan(0);
  });

  it("handles equal fields", () => {
    expect(compareFields("TimeReceived", "TimeReceived")).toBe(0);
    expect(compareFields("SrcAddr", "SrcAddr")).toBe(0);
  });
});
