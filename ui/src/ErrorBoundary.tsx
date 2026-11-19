import { Component, type ReactNode } from "react";
import { t } from "./i18n";

interface State { error: Error | null }

/** Menangkap error render satu halaman supaya sisanya (menu, keluar) tetap hidup dan pesannya terlihat. */
export default class ErrorBoundary extends Component<{ children: ReactNode; resetKey?: string }, State> {
  state: State = { error: null };
  static getDerivedStateFromError(error: Error): State {
    return { error };
  }
  componentDidCatch(error: Error, info: { componentStack?: string | null }) {
    console.error("panel: error render", error, info.componentStack);
  }
  componentDidUpdate(prev: { resetKey?: string }) {
    if (prev.resetKey !== this.props.resetKey && this.state.error) this.setState({ error: null });
  }
  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="card stack" role="alert">
        <h2 className="sec-title">{t("err.title")}</h2>
        <p className="small muted">{t("err.body")}</p>
        <p className="mono small">{this.state.error.message}</p>
        <div><button className="btn" onClick={() => location.reload()}>{t("err.reload")}</button></div>
      </div>
    );
  }
}
