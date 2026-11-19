import ProfileCard from "../features/profile/ProfileCard";
import { PageHead } from "../ui";
import { useT } from "../i18n";
import Account from "./Account";
import { TwoFactor } from "./Settings";

export default function Profile() {
  const t = useT();
  return (
    <>
      <PageHead title={t("profile.mine")} desc={t("settings.profile.desc")} />
      <div className="stack">
        <ProfileCard />
        <Account />
        <TwoFactor />
      </div>
    </>
  );
}
