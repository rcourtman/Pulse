name: Update date_created in PR JSON files

on:
  pull_request:
    types: [opened, synchronize]
  schedule:
    - cron: "0 0,6,12,18 * * *"  
  workflow_dispatch:  

jobs:
  update-date:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout PR
        uses: actions/checkout@v4
        with:
          ref: ${{ github.head_ref }}  

      - name: Install yq
        run: |
          sudo apt-get update
          sudo apt-get install -y jq

      - name: Update date in JSON
        run: |
          ./update_json_date.sh
