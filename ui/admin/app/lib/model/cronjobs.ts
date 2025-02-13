import { EntityMeta } from "~/lib/model/primitives";

export type CronJobBase = {
	description: string;
	workflow: string;
	schedule?: string; // cron string
	taskSchedule?: {
		interval: "hourly" | "monthly" | "daily" | "weekly";
		day: number;
		hour: number;
		minute: number;
		weekday: number;
		timezone: string;
	};
	timezone: string;
	input?: string;
};

export type CronJob = EntityMeta & CronJobBase;
