// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// See https://design.gitlab.com/data-visualization/color
const colors = {
  blue: [
    "#e9ebff", // 50
    "#d4dcfa", // 100
    "#b7c6ff", // 200
    "#97acff", // 300
    "#748eff", // 400
    "#5772ff", // 500
    "#445cf2", // 600
    "#3547de", // 700
    "#232fcf", // 800
    "#1e23a8", // 900
    "#11118a", // 950
  ],
  orange: [
    "#fae8d1",
    "#f7d8b5",
    "#f3c291",
    "#eb9a5c",
    "#e17223",
    "#d14e00",
    "#b24800",
    "#944100",
    "#6f3500",
    "#5c2b00",
    "#421e00",
  ],
  aqua: [
    "#b8fff2",
    "#93fae7",
    "#5eebdf",
    "#25d2d2",
    "#0bb6c6",
    "#0094b6",
    "#0080a1",
    "#006887",
    "#004d67",
    "#003f57",
    "#00293d",
  ],
  green: [
    "#ddfab7",
    "#c9f097",
    "#b0de73",
    "#94c25e",
    "#83ab4a",
    "#608b2f",
    "#487900",
    "#366800",
    "#275600",
    "#1a4500",
    "#0f3300",
  ],
  magenta: [
    "#ffe3eb",
    "#ffc9d9",
    "#fcacc5",
    "#ff85af",
    "#f2639a",
    "#d84280",
    "#c52c6b",
    "#b31756",
    "#950943",
    "#7a0033",
    "#570028",
  ],
};

const orderedColors = ["blue", "orange", "aqua", "green", "magenta"] as const;

const darkPalette = [5, 6, 7, 8, 9, 10]
  .map((idx) =>
    orderedColors.map(
      (colorName: keyof typeof colors) => colors[colorName][idx]
    )
  )
  .flat();
const lightPalette = [5, 4, 3, 2, 1, 0]
  .map((idx) => orderedColors.map((colorName) => colors[colorName][idx]))
  .flat();
const lightenColor = (color: string, amount: number) =>
  "#" +
  color
    .replace(/^#/, "")
    .replace(/../g, (color) =>
      (
        "0" +
        Math.min(255, Math.max(0, parseInt(color, 16) + amount)).toString(16)
      ).slice(-2)
    );

export function dataColor(
  index: number,
  alternate = false,
  theme: "light" | "dark" = "light"
) {
  const palette = theme === "light" ? lightPalette : darkPalette;
  const correctedIndex = index % 2 === 0 ? index : index + orderedColors.length;
  const computed = palette[correctedIndex % palette.length];
  if (!alternate) {
    return computed;
  }
  return lightenColor(computed, 20);
}

export function dataColorGrey(
  index: number,
  alternate = false,
  theme: "light" | "dark" = "light"
) {
  const palette =
    theme === "light"
      ? ["#aaaaaa", "#bbbbbb", "#999999", "#cccccc", "#888888"]
      : ["#666666", "#777777", "#555555", "#888888", "#444444"];
  const computed = palette[index % palette.length];
  if (!alternate) {
    return computed;
  }
  return lightenColor(computed, 10);
}
