import { SiteDetail } from "../features/sites/SiteDetail";
import { SiteList } from "../features/sites/SiteList";

export default function Sites({ sub }: { sub: string }) {
  return sub ? <SiteDetail name={sub} /> : <SiteList />;
}
