import { get } from "../../api";
import { usePoll } from "../../ui";
import type { RangeKey } from "../../lib";
import type { HwData } from "./model";

/** Perangkat keras dari /api/hw, disegarkan tiap 30 detik (sama dengan interval sampel panel). */
export const useHw = (range: RangeKey) => usePoll(() => get<HwData>(`/api/hw?range=${range}`), 30000, [range]);
