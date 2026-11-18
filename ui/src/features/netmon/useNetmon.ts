import { get } from "../../api";
import { usePoll } from "../../ui";
import type { RangeKey } from "../../lib";
import type { NetData } from "./model";

/** Konektivitas dari /api/netmon, disegarkan tiap 60 detik (interval sampel panel). */
export const useNetmon = (range: RangeKey) => usePoll(() => get<NetData>(`/api/netmon?range=${range}`), 60000, [range]);
