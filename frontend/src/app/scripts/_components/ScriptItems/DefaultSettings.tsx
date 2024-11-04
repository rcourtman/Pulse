import { Script } from "@/lib/types";

export default function DefaultSettings({ item }: { item: Script }) {
  const hasAlpineScript = item?.expand?.alpine_script !== undefined;

  return (
    <>
      {item.default_cpu && (
        <div>
          <h2 className="text-md font-semibold">Default settings</h2>
          <p className="text-sm text-muted-foreground">
            CPU: {item.default_cpu}
          </p>
          <p className="text-sm text-muted-foreground">
            RAM: {item.default_ram}
          </p>
          <p className="text-sm text-muted-foreground">
            HDD: {item.default_hdd}
          </p>
        </div>
      )}
      {hasAlpineScript && (
        <div>
          <h2 className="text-md font-semibold">Default Alpine settings</h2>
          <p className="text-sm text-muted-foreground">
            CPU: {item.expand.alpine_script.default_cpu}
          </p>
          <p className="text-sm text-muted-foreground">
            RAM: {item.expand.alpine_script.default_ram}
          </p>
          <p className="text-sm text-muted-foreground">
            HDD: {item.expand.alpine_script.default_hdd}
          </p>
        </div>
      )}
    </>
  );
}
