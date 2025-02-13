import { GlobeIcon } from "lucide-react";

import { Label } from "~/components/ui/label";

type TimezoneSelectionProps = {
	timezone: string;
};

export function TimezoneSelection({ timezone }: TimezoneSelectionProps) {
	return (
		<div className="flex-r flex items-center">
			{timezone && (
				<>
					<GlobeIcon className="mr-2 h-4 w-4 text-muted-foreground" />
					<Label className="text-sm text-muted-foreground">{timezone}</Label>
				</>
			)}
		</div>
	);
}
