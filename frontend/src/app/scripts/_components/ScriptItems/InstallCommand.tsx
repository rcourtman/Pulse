import CodeCopyButton from "@/components/ui/code-copy-button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Script } from "@/lib/types";

export default function InstallCommand({ item }: { item: Script }) {
  const { title, item_type, installCommand, expand } = item;
  const hasAlpineScript = expand?.alpine_script !== undefined;

  const renderInstructions = (isAlpine = false) => (
    <>
      <p className="text-sm mt-2">
        {isAlpine ? (
          <>
            As an alternative option, you can use Alpine Linux and the {title}{" "}
            package to create a {title} {item_type} container with faster
            creation time and minimal system resource usage. You are also
            obliged to adhere to updates provided by the package maintainer.
          </>
        ) : item_type ? (
          <>
            To create a new Proxmox VE {title} {item_type}, run the command
            below in the Proxmox VE Shell.
          </>
        ) : (
          <>To use the {title} script, run the command below in the shell.</>
        )}
      </p>
      {isAlpine && (
        <p className="mt-2 text-sm">
          To create a new Proxmox VE Alpine-{title} {item_type}, run the command
          below in the Proxmox VE Shell
        </p>
      )}
    </>
  );

  return (
    <div className="p-4">
      {hasAlpineScript ? (
        <Tabs defaultValue="default" className="mt-2 w-full max-w-4xl">
          <TabsList>
            <TabsTrigger value="default">Default</TabsTrigger>
            <TabsTrigger value="alpine">Alpine Linux</TabsTrigger>
          </TabsList>
          <TabsContent value="default">
            {renderInstructions()}
            <CodeCopyButton>{installCommand}</CodeCopyButton>
          </TabsContent>
          <TabsContent value="alpine">
            {expand.alpine_script && (
              <>
                {renderInstructions(true)}
                <CodeCopyButton>
                  {expand.alpine_script.installCommand}
                </CodeCopyButton>
              </>
            )}
          </TabsContent>
        </Tabs>
      ) : (
        <>
          {renderInstructions()}
          {installCommand && <CodeCopyButton>{installCommand}</CodeCopyButton>}
        </>
      )}
    </div>
  );
}
