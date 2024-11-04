// these are all the interfaces that are used in the site. these all come from the pocketbase database

export interface Script {
  title: string;
  description: string;
  documentation: string;
  website: string;
  logo: string;
  created: string;
  updated: string;
  id: string;
  item_type: string;
  interface: string;
  installCommand: string;
  port: number;
  post_install: string;
  default_cpu: string;
  default_hdd: string;
  default_ram: string;
  isUpdateable: boolean;
  isMostViewed: boolean;
  privileged: boolean;
  alpineScript: alpine_script;
  expand: {
    alpine_script: alpine_script;
    alerts: alerts[];
    default_login: default_login;
  };
}

export interface Category {
  catagoryName: string;
  categoryId: string;
  id: string;
  created: string;
  expand: {
    items: Script[];
  };
}

interface alpine_script {
  installCommand: string;
  default_cpu: string;
  default_hdd: string;
  default_ram: string;
}

interface alerts {
  content: string;
}

interface default_login {
  username: string;
  password: string;
}
