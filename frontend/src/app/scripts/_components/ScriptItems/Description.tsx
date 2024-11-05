import TextCopyBlock from "@/components/TextCopyBlock";
import { Script } from "@/lib/types";

export default function Description({ item }: { item: Script }) {
  return (
    <div className="p-2">
      <h2 className="mb-2 max-w-prose text-lg font-semibold">Description</h2>
      <p className="text-sm text-muted-foreground">
        {TextCopyBlock(item.description)}
      </p>
    </div>
  );
}
