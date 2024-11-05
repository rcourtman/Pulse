import type { MetadataRoute } from "next";

export const generateStaticParams = () => {
  return [];
};

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Proxmox VE Helper-Scripts",
    short_name: "Proxmox VE Helper-Scripts",
    description:
      "A Re-designed Front-end for the Proxmox VE Helper-Scripts Repository. Featuring over 150+ scripts to help you manage your Proxmox VE environment.",
    theme_color: "#030712",
    background_color: "#030712",
    display: "standalone",
    orientation: "portrait",
    scope: "/Proxmox/",
    start_url: "/Proxmox/",
    icons: [
      {
        src: "logo.png",
        sizes: "512x512",
        type: "image/png",
      },
    ],
  };
}
