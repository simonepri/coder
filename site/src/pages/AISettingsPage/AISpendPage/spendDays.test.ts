import { describe, expect, it } from "vitest";
import {
	defaultSpendDays,
	formatSpendDays,
	fromPickerValue,
	parseSpendDays,
	toPickerValue,
	toSpendPeriod,
} from "./spendDays";

describe("defaultSpendDays", () => {
	it("covers the last 7 days, including today", () => {
		expect(defaultSpendDays(new Date(2026, 2, 12, 15, 30))).toEqual({
			startDate: "2026-03-06",
			endDate: "2026-03-12",
		});
	});
});

describe("parseSpendDays", () => {
	it.each([
		["a single day", "2026-03-12", "2026-03-12"],
		["the longest range", "2026-01-01", "2026-01-31"],
	])("accepts %s", (_, startDate, endDate) => {
		expect(parseSpendDays(startDate, endDate)).toEqual({ startDate, endDate });
	});

	it.each([
		["a missing start", null, "2026-03-12"],
		["a missing end", "2026-03-12", null],
		["a timestamp", "2026-03-06T00:00:00Z", "2026-03-12"],
		["an impossible date", "2026-02-30", "2026-03-12"],
		["a reversed range", "2026-03-12", "2026-03-06"],
		["a range over 31 days", "2026-01-01", "2026-02-01"],
	])("rejects %s", (_, startDate, endDate) => {
		expect(parseSpendDays(startDate, endDate)).toBeNull();
	});
});

describe("toSpendPeriod", () => {
	it("bounds the days by UTC midnights with an exclusive end", () => {
		expect(
			toSpendPeriod({ startDate: "2026-03-06", endDate: "2026-03-12" }),
		).toEqual({
			period_start: "2026-03-06T00:00:00.000Z",
			period_end: "2026-03-13T00:00:00.000Z",
		});
	});
});

describe("formatSpendDays", () => {
	it.each([
		["a single day", "2026-03-12", "2026-03-12", "Mar 12"],
		["days in one month", "2026-03-06", "2026-03-12", "Mar 6-12"],
		["days across months", "2026-02-27", "2026-03-12", "Feb 27 - Mar 12"],
		["days across years", "2025-12-29", "2026-01-04", "Dec 29 - Jan 4"],
	])("labels %s", (_, startDate, endDate, label) => {
		expect(formatSpendDays({ startDate, endDate })).toBe(label);
	});
});

describe("fromPickerValue", () => {
	it("reads the last day from a range ending at midnight", () => {
		expect(
			fromPickerValue({
				startDate: new Date(2026, 2, 6),
				endDate: new Date(2026, 2, 13),
			}),
		).toEqual({ startDate: "2026-03-06", endDate: "2026-03-12" });
	});

	it("reads today from a range ending at the next hour", () => {
		expect(
			fromPickerValue({
				startDate: new Date(2026, 2, 6),
				endDate: new Date(2026, 2, 12, 16),
			}),
		).toEqual({ startDate: "2026-03-06", endDate: "2026-03-12" });
	});

	it("round-trips the days through the picker", () => {
		const days = { startDate: "2026-02-27", endDate: "2026-03-12" };
		expect(fromPickerValue(toPickerValue(days))).toEqual(days);
	});
});
