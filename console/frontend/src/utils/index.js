export function formatBps(value) {
  const suffixes = ["", "K", "M", "G", "T"];
  let idx = 0;
  while (value >= 1000 && idx < suffixes.length) {
    value /= 1000;
    idx++;
  }
  value = value.toFixed(2);
  return `${value}${suffixes[idx]}`;
}

export { dataColor, dataColorGrey } from "./palette.js";
