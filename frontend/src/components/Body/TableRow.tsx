import { createSignal, Show, For } from "solid-js";
import { editNames, selectedIDs, setSelectedIDs } from "../../functions/exports";
import { apiEditHost } from "../../functions/api";
import { vendorIcon } from "../../functions/vendor";

import { debounce } from "@solid-primitives/scheduled"; 

function TableRow(_props: any) {

  const [name, setName] = createSignal(_props.host.Name);

  const ports = (_props.host.Ports || "").split(",").filter((p: string) => p !== "");

  const displayName = () => {
    const currentName = name().trim();
    if (currentName !== "") {
      return currentName;
    }

    const hwApi = (_props.host.HwApi || "").trim();
    if (hwApi !== "") {
      return hwApi;
    }

    const hardware = (_props.host.Hw || "").trim();
    if (hardware !== "" && !hardware.startsWith("(Unknown")) {
      return hardware;
    }

    const mac = (_props.host.Mac || "").trim();
    if (mac !== "") {
      return "Unknown " + mac.slice(-5);
    }

    return "Unknown";
  };
  
  let now = <i class="bi bi-circle-fill" style="color:var(--bs-gray-500);"></i>;
  if (_props.host.Now == 1) {
    now = <i class="bi bi-check-circle-fill" style="color:var(--bs-success);"></i>;
  };

  let known:boolean;
  _props.host.Known === 1 ? known = true : known = false;

  const debouncedApi = debounce(async (val: string) => {
    await apiEditHost(_props.host.ID, val, "");
  }, 300);

  const handleInput = async (n: string) => {
    setName(n);
    debouncedApi(n);
  };
  const handleToggle = async () => {
    await apiEditHost(_props.host.ID, name(), "toggle");
  };

  const handleCheck = (checked: boolean) => {
    const id = _props.host.ID;
    setSelectedIDs(prev => {
      if (checked) {
        return prev.includes(id) ? prev : [...prev, id];
      } else {
        return prev.filter(item => item !== id);
      }
    });
  };

  return (
    <tr>
      <td class="opacity-50">{_props.index}.</td>
      <td>
        <Show
          when={editNames()}
          fallback={
            <span class={name().trim() === "" ? "text-muted" : ""}>{displayName()}</span>
          }
        >
          <input type="text" class="form-control" value={name()}
            onInput={e => handleInput(e.target.value)}></input>
        </Show>
      </td>
      <td>{_props.host.Iface}</td>
      <td><a href={"http://" + _props.host.IP} target="_blank">{_props.host.IP}</a></td>
      <td>{_props.host.Mac}</td>
      <td title={_props.host.Hw}>
        <i class={"bi " + vendorIcon(_props.host.Hw)} style="opacity:0.8;"></i>
        &nbsp;{_props.host.Hw}
      </td>
      <td title={_props.host.HwApi}>
        <i class={"bi " + vendorIcon(_props.host.HwApi)} style="opacity:0.8;"></i>
        &nbsp;{_props.host.HwApi}
      </td>
      <td title={_props.host.OsName}>{_props.host.OsName}</td>
      <td title={_props.host.DevType}>{_props.host.DevType}</td>
      <td>
        <For each={ports}>{(port) =>
          <a class="badge text-bg-secondary me-1 text-decoration-none" href={"http://" + _props.host.IP + ":" + port} target="_blank">{port}</a>
        }</For>
      </td>
      <td>{_props.host.Date}</td>
      <td>
        <div class="form-check form-switch">
          <input class="form-check-input" type="checkbox" checked={known}
            onClick={handleToggle}></input>
        </div>
      </td>
      <td>{now}</td>
      <td>
        <Show
          when={editNames()}
          fallback={
          <a href={"/host/" + _props.host.ID}>
            <i class="bi bi-three-dots-vertical my-btn p-2" title="More"></i>
          </a>}
        >
          <input
            type="checkbox"
            class="form-check-input"
            checked={selectedIDs().includes(_props.host.ID)}
            onChange={e => handleCheck((e.target as HTMLInputElement).checked)}
          />
        </Show>
      </td>
    </tr>
  )
}

export default TableRow
