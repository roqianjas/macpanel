import ProfileCard from "../features/profile/ProfileCard";
import { PageHead } from "../ui";
import Account from "./Account";
import { TwoFactor } from "./Settings";

export default function Profile() {
  return (
    <>
      <PageHead title="Profil saya" desc="Nama, avatar, kata sandi, dan verifikasi dua langkah (2FA) untuk akun Anda sendiri." />
      <div className="stack">
        <ProfileCard />
        <Account />
        <TwoFactor />
      </div>
    </>
  );
}
