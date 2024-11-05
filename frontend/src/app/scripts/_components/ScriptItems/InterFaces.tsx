import { Button, buttonVariants } from "@/components/ui/button";
import handleCopy from "@/components/handleCopy";
import { cn } from "@/lib/utils";
import { ClipboardIcon } from "lucide-react";

interface Item {
  interface?: string;
  port?: number;
}

const CopyButton = ({
  label,
  value,
}: {
  label: string;
  value: string | number;
}) => (
  <span className={cn(buttonVariants({size: "sm", variant: "secondary"}), "flex items-center gap-2")}>
    {value}
    <ClipboardIcon
      onClick={() => handleCopy(label, String(value))}
      className="size-4 cursor-pointer"
    />
  </span>
);

export default function InterFaces({ item }: { item: Item }) {
  const { interface: iface, port } = item;

  return (
    <div className="flex flex-col gap-2">
      {iface || (port && port !== 0) ? (
        <div className="flex items-center justify-end">
          <h2 className="mr-2 text-end text-lg font-semibold">
            {iface ? "Interface:" : "Default Port:"}
          </h2>{" "}
          <CopyButton
            label={iface ? "interface" : "port"}
            value={iface || port!}
          />
        </div>
      ) : null}
    </div>
  );
}
