// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it } from "vitest";
import { dataColor, dataColorGrey } from "./palette";

describe("dataColor", () => {
  it("returns valid hex colors", () => {
    expect(dataColor(0)).toMatch(/^#[0-9a-f]{6}$/);
    expect(dataColor(1)).toMatch(/^#[0-9a-f]{6}$/);
    expect(dataColor(100)).toMatch(/^#[0-9a-f]{6}$/);
  });

  it("returns different colors for light and dark themes", () => {
    expect(dataColor(1, false, "light")).not.toBe(dataColor(1, false, "dark"));
    expect(dataColor(3, false, "light")).not.toBe(dataColor(3, false, "dark"));
  });

  it("handles index wrapping consistently", () => {
    const color0 = dataColor(0);
    const color30 = dataColor(30);
    expect(color30).toBe(color0);
  });

  it("applies different indexing for odd vs even indices", () => {
    const evenIndex = dataColor(2);
    const oddIndex = dataColor(3);
    const oddIndexPlus5 = dataColor(8);
    expect(evenIndex).not.toBe(oddIndex);
    expect(oddIndex).toBe(oddIndexPlus5);
  });

  it("returns lightened colors when alternate is true", () => {
    const original = dataColor(0, false);
    const lightened = dataColor(0, true);
    expect(lightened).not.toBe(original);
    expect(lightened).toMatch(/^#[0-9a-f]{6}$/);
  });

  it("applies alternates consistently across themes", () => {
    const lightOriginal = dataColor(0, false, "light");
    const lightAlternate = dataColor(0, true, "light");
    const darkOriginal = dataColor(0, false, "dark");
    const darkAlternate = dataColor(0, true, "dark");

    expect(lightAlternate).not.toBe(lightOriginal);
    expect(darkAlternate).not.toBe(darkOriginal);
  });
});

describe("dataColorGrey", () => {
  it("returns valid hex colors", () => {
    expect(dataColorGrey(0)).toMatch(/^#[0-9a-f]{6}$/);
    expect(dataColorGrey(1)).toMatch(/^#[0-9a-f]{6}$/);
    expect(dataColorGrey(100)).toMatch(/^#[0-9a-f]{6}$/);
  });

  it("returns different colors for light and dark themes", () => {
    expect(dataColorGrey(0, false, "light")).not.toBe(
      dataColorGrey(0, false, "dark"),
    );
    expect(dataColorGrey(2, false, "light")).not.toBe(
      dataColorGrey(2, false, "dark"),
    );
  });

  it("handles index wrapping with 5-color palette", () => {
    expect(dataColorGrey(5)).toBe(dataColorGrey(0));
    expect(dataColorGrey(10)).toBe(dataColorGrey(0));
    expect(dataColorGrey(7)).toBe(dataColorGrey(2));
  });

  it("returns lightened colors when alternate is true", () => {
    const original = dataColorGrey(0);
    const lightened = dataColorGrey(0, true);
    expect(lightened).not.toBe(original);
    expect(lightened).toMatch(/^#[0-9a-f]{6}$/);
  });

  it("applies alternates consistently across themes", () => {
    const lightOriginal = dataColorGrey(0, false, "light");
    const lightAlternate = dataColorGrey(0, true, "light");
    const darkOriginal = dataColorGrey(0, false, "dark");
    const darkAlternate = dataColorGrey(0, true, "dark");

    expect(lightAlternate).not.toBe(lightOriginal);
    expect(darkAlternate).not.toBe(darkOriginal);
  });

  it("produces different results than dataColor", () => {
    expect(dataColorGrey(0)).not.toBe(dataColor(0));
    expect(dataColorGrey(1)).not.toBe(dataColor(1));
  });
});
