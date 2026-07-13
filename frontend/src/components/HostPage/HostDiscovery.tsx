import { createSignal, Show, For } from "solid-js";
import { apiHostDiscovery } from "../../functions/api";
import { HostInfo } from "../../functions/exports";

export function HostDiscovery(props: { ip: string }) {
  const [info, setInfo] = createSignal<HostInfo | null>(null);
  const [loading, setLoading] = createSignal(false);

  const run = async () => {
    setLoading(true);
    try {
      setInfo(await apiHostDiscovery(props.ip));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div class="card mt-3">
      <div class="card-header d-flex justify-content-between align-items-center">
        <span>Discovery details</span>
        <button class="btn btn-sm btn-outline-secondary" onClick={run} disabled={loading()}>
          <Show when={!loading()} fallback="Scanning…">
            <i class="bi bi-arrow-clockwise"></i> Scan now
          </Show>
        </button>
      </div>
      <div class="card-body">
        <Show
          when={info()}
          fallback={<small class="text-muted">Click "Scan now" to query mDNS / SSDP / UPnP for this device.</small>}
        >
          {(data) => (
            <table class="table table-sm">
              <tbody>
                <Show when={data().Manufacturer}>
                  <tr><td>Manufacturer</td><td>{data().Manufacturer}</td></tr>
                </Show>
                <Show when={data().Model}>
                  <tr><td>Model</td><td>{data().Model}</td></tr>
                </Show>
                <Show when={data().ModelNumber}>
                  <tr><td>Model number</td><td>{data().ModelNumber}</td></tr>
                </Show>
                <Show when={data().ModelDescription}>
                  <tr><td>Description</td><td>{data().ModelDescription}</td></tr>
                </Show>
                <Show when={data().Serial}>
                  <tr><td>Serial</td><td>{data().Serial}</td></tr>
                </Show>
                <Show when={data().DeviceType}>
                  <tr><td>Type</td><td>{data().DeviceType}</td></tr>
                </Show>
                <Show when={data().PresentationURL}>
                  <tr>
                    <td>Web UI</td>
                    <td><a href={data().PresentationURL} target="_blank">{data().PresentationURL}</a></td>
                  </tr>
                </Show>
                <Show when={data().Services && data().Services.length > 0}>
                  <tr>
                    <td>Services</td>
                    <td><For each={data().Services}>{(s) => <span class="badge text-bg-secondary me-1">{s}</span>}</For></td>
                  </tr>
                </Show>
              </tbody>
            </table>
          )}
        </Show>
      </div>
    </div>
  );
}
