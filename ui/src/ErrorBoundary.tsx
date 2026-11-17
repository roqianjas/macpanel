import { Component, type ReactNode } from "react";

interface State { error: Error | null }

/** Menangkap error render satu halaman supaya sisanya (menu, keluar) tetap hidup dan pesannya terlihat. */
export default class ErrorBoundary extends Component<{ children: ReactNode; resetKey?: string }, State> {
  state: State = { error: null };
  static getDerivedStateFromError(error: Error): State {
    return { error };
  }
  componentDidUpdate(prev: { resetKey?: string }) {
    if (prev.resetKey !== this.props.resetKey && this.state.error) this.setState({ error: null });
  }
  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="card stack" role="alert">
        <h2 className="sec-title">Halaman ini gagal ditampilkan</h2>
        <p className="small muted">Terjadi kesalahan di tampilan, bukan di server Anda. Muat ulang halaman; bila berulang, laporkan pesan di bawah.</p>
        <p className="mono small">{this.state.error.message}</p>
        <div><button className="btn" onClick={() => location.reload()}>Muat ulang</button></div>
      </div>
    );
  }
}
