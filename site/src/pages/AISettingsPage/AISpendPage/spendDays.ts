import dayjs from "dayjs";
import utc from "dayjs/plugin/utc";
import {
	type AISpendPeriodWindow,
	MaxAISpendPeriodDays,
} from "#/api/typesGenerated";
import type { DateRangeValue } from "#/components/DateRangePicker/DateRangePicker";
import { DATE_FORMAT } from "#/utils/time";

dayjs.extend(utc);

const DEFAULT_SPEND_DAY_COUNT = 7;

/** An inclusive range of calendar days, each formatted as `YYYY-MM-DD`. */
export type SpendDays = {
	startDate: string;
	endDate: string;
};

/** The default range: the last 7 days, including today. */
export const defaultSpendDays = (now: Date): SpendDays => ({
	startDate: dayjs(now)
		.subtract(DEFAULT_SPEND_DAY_COUNT - 1, "day")
		.format(DATE_FORMAT.ISO_DATE),
	endDate: dayjs(now).format(DATE_FORMAT.ISO_DATE),
});

const isDay = (value: string): boolean =>
	dayjs.utc(value).format(DATE_FORMAT.ISO_DATE) === value;

/**
 * Reads a range from URL values, or returns null when they are missing,
 * malformed, reversed, or longer than the server allows.
 */
export const parseSpendDays = (
	startDate: string | null,
	endDate: string | null,
): SpendDays | null => {
	if (!startDate || !endDate || !isDay(startDate) || !isDay(endDate)) {
		return null;
	}
	const dayCount = dayjs.utc(endDate).diff(dayjs.utc(startDate), "day") + 1;
	if (dayCount < 1 || dayCount > MaxAISpendPeriodDays) {
		return null;
	}
	return { startDate, endDate };
};

/**
 * Spend is aggregated per UTC day, so each picked calendar day maps to that
 * UTC day. The API takes an exclusive end.
 */
export const toSpendPeriod = (days: SpendDays): AISpendPeriodWindow => ({
	period_start: dayjs.utc(days.startDate).toISOString(),
	period_end: dayjs.utc(days.endDate).add(1, "day").toISOString(),
});

/**
 * Labels the days compactly, e.g. "Mar 6-12" or "Feb 27 - Mar 12".
 */
export const formatSpendDays = ({ startDate, endDate }: SpendDays): string => {
	const start = dayjs.utc(startDate);
	const end = dayjs.utc(endDate);
	if (startDate === endDate) {
		return start.format("MMM D");
	}
	if (start.isSame(end, "month")) {
		return `${start.format("MMM D")}-${end.format("D")}`;
	}
	return `${start.format("MMM D")} - ${end.format("MMM D")}`;
};

/** Converts the days into the picker's local, end-exclusive boundaries. */
export const toPickerValue = (days: SpendDays): DateRangeValue => ({
	startDate: dayjs(days.startDate).toDate(),
	endDate: dayjs(days.endDate).add(1, "day").toDate(),
});

/**
 * Reads the picked calendar days back from the picker. The picker ends a range
 * at the next local midnight, or at the next hour when the range ends today.
 */
export const fromPickerValue = (value: DateRangeValue): SpendDays => {
	const end = dayjs(value.endDate);
	const lastDay = end.isSame(end.startOf("day")) ? end.subtract(1, "day") : end;
	return {
		startDate: dayjs(value.startDate).format(DATE_FORMAT.ISO_DATE),
		endDate: lastDay.format(DATE_FORMAT.ISO_DATE),
	};
};
