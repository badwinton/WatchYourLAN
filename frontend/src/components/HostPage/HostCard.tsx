import { For, Show } from "solid-js";
import { apiDelHost, apiEditHost, apiRefreshOUI, apiWOL } from "../../functions/api";
import { vendorIcon } from "../../functions/vendor";

import { debounce } from "@solid-primitives/scheduled"; 

function HostCard(_props: any) {

  let name:string = "";

  const ports = (_props.host.Ports || "").split(",").filter((p: string) => p !== "");

  const debouncedApi = debounce(async (val: string) => {
      await apiEditHost(_props.host.ID, val, "");
    }, 300);

  const handleInput = async (n: string) => {
    
    name = n;
    debouncedApi(n);
  };

  const handleToggle = async () => {

    if (name == "") {
      name = _props.host.Name;
    }

    await apiEditHost(_props.host.ID, name, 'toggle');
  };

  const handleDel = async () => {
    
    await apiDelHost(_props.host.ID);
    window.location.href = '/';
  };

  const handleWOL = async () => {
    
    await apiWOL(_props.host.Mac);
  };

  const handleRefreshOUI = async () => {

    const updated = await apiRefreshOUI(_props.host.ID);
    _props.host.Hw = updated.Hw;
  };

  return (
    <div class="card border-primary">
      <div class="card-header">Host</div>
      <div class="card-body table-responsive">
        <table class="table table-striped table-hover">
          <tbody>
          <tr>
            <td>ID</td>
            <td>{_props.host.ID}</td>
          </tr>
          <tr>
            <td>Name</td>
            <td>
            <input type="text" class="form-control" value={_props.host.Name}
              onInput={e => handleInput(e.target.value)}></input>
            </td>
          </tr>
          <tr>
            <td>DNS name</td>
            <td>{_props.host.DNS}</td>
          </tr>
          <tr>
            <td>Iface</td>
            <td>{_props.host.Iface}</td>
          </tr>
          <tr>
            <td>IP</td>
            <td>
              <a href={"http://" + _props.host.IP} target="_blank">{_props.host.IP}</a>
            </td>
          </tr>
          <tr>
            <td>MAC</td>
            <td>{_props.host.Mac}</td>
          </tr>
          <tr>
            <td>ARP Vendor</td>
            <td>
              <i class={"bi " + vendorIcon(_props.host.Hw)} style="opacity:0.8;"></i>
              &nbsp;{_props.host.Hw}
            </td>
          </tr>
          <tr>
            <td>API Vendor</td>
            <td>
              <i class={"bi " + vendorIcon(_props.host.HwApi)} style="opacity:0.8;"></i>
              &nbsp;{_props.host.HwApi}
            </td>
          </tr>
          <tr>
            <td>Open ports</td>
            <td>
              <For each={ports}>{(port) =>
                <a class="badge text-bg-secondary me-1 text-decoration-none" href={"http://" + _props.host.IP + ":" + port} target="_blank">{port}</a>
              }</For>
            </td>
          </tr>
          <Show when={_props.host.OsName}>
          <tr>
            <td>OS</td>
            <td>{_props.host.OsName}</td>
          </tr>
          </Show>
          <Show when={_props.host.DevType}>
          <tr>
            <td>Device type</td>
            <td>{_props.host.DevType}</td>
          </tr>
          </Show>
          <Show when={_props.host.Services}>
          <tr>
            <td>Services</td>
            <td><small>{_props.host.Services}</small></td>
          </tr>
          </Show>
          <tr>
            <td>Date</td>
            <td>{_props.host.Date}</td>
          </tr>
          <tr>
            <td>Known</td>
            <td>
              <div class="form-check form-switch">
                <input class="form-check-input" type="checkbox" 
                    onClick={handleToggle}
                    checked={_props.host.Known == 1
                      ? true
                      : false
                    }
                    ></input>
              </div>
            </td>
          </tr>
          <tr>
            <td>Online</td>
            <td>{_props.host.Now == 1
                  ? <i class="bi bi-check-circle-fill" style="color:var(--bs-success);"></i>
                  : <i class="bi bi-circle-fill" style="color:var(--bs-gray-500);"></i>
                }
                &nbsp;&nbsp;&nbsp;
                <button type="button" onClick={handleWOL} class="btn btn-outline-success">Wake-on-LAN</button>
            </td>
          </tr>
          </tbody>
        </table>
        <button type="button" onClick={handleDel} class="btn btn-outline-danger">Delete host</button>
        &nbsp;
        <button type="button" onClick={handleRefreshOUI} class="btn btn-outline-primary">Refresh Vendor</button>
      </div>
    </div>
  )
}

export default HostCard