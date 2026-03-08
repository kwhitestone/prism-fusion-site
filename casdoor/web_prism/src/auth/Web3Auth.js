// // Copyright 2023 The Casdoor Authors. All Rights Reserved.
// //
// // Licensed under the Apache License, Version 2.0 (the "License");
// // you may not use this file except in compliance with the License.
// // You may obtain a copy of the License at
// //
// //      http://www.apache.org/licenses/LICENSE-2.0
// //
// // Unless required by applicable law or agreed to in writing, software
// // distributed under the License is distributed on an "AS IS" BASIS,
// // WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// // See the License for the specific language governing permissions and
// // limitations under the License.

// Web3 functionality is temporarily disabled to reduce build size.
// To re-enable, restore the original imports from @web3-onboard/*, @metamask/eth-sig-util, etc.

import {showMessage} from "../Setting";
import {v4 as uuidv4} from "uuid";

export function generateNonce() {
  const nonce = uuidv4();
  return nonce;
}

export function getWeb3AuthTokenKey(address) {
  return `Web3AuthToken_${address}`;
}

export function setWeb3AuthToken(token) {
  const key = getWeb3AuthTokenKey(token.address);
  localStorage.setItem(key, JSON.stringify(token));
}

export function getWeb3AuthToken(address) {
  const key = getWeb3AuthTokenKey(address);
  return JSON.parse(localStorage.getItem(key));
}

export function delWeb3AuthToken(address) {
  const key = getWeb3AuthTokenKey(address);
  localStorage.removeItem(key);
}

export function clearWeb3AuthToken() {
  const keys = Object.keys(localStorage);
  keys.forEach(key => {
    if (key.startsWith("Web3AuthToken_")) {
      localStorage.removeItem(key);
    }
  });
}

export function detectMetaMaskPlugin() {
  // check if ethereum extension MetaMask is installed
  return window.ethereum && window.ethereum.isMetaMask;
}

export function requestEthereumAccount() {
  const method = "eth_requestAccounts";
  const selectedAccount = window.ethereum.request({method})
    .then((accounts) => {
      return accounts[0];
    });
  return selectedAccount;
}

export function signEthereumTypedData(from, nonce) {
  // https://docs.metamask.io/wallet/how-to/sign-data/
  const date = new Date();
  const typedData = JSON.stringify({
    domain: {
      chainId: window.ethereum.chainId,
      name: "PrismAccount",
      version: "1",
    },
    message: {
      prompt: "In order to authenticate to this website, sign this request and your public address will be sent to the server in a verifiable way.",
      nonce: nonce,
      createAt: `${date.toLocaleString()}`,
    },
    primaryType: "AuthRequest",
    types: {
      EIP712Domain: [
        {name: "name", type: "string"},
        {name: "version", type: "string"},
        {name: "chainId", type: "uint256"},
      ],
      AuthRequest: [
        {name: "prompt", type: "string"},
        {name: "nonce", type: "string"},
        {name: "createAt", type: "string"},
      ],
    },
  });

  const method = "eth_signTypedData_v4";
  const params = [from, typedData];

  return window.ethereum.request({method, params})
    .then((sign) => {
      return {
        address: from,
        createAt: Math.floor(date.getTime() / 1000),
        typedData: typedData,
        signature: sign,
      };
    });
}

export function checkEthereumSignedTypedData(token) {
  showMessage("error", "Web3 functionality is temporarily disabled");
  return false;
}

export async function authViaMetaMask(application, provider, method) {
  showMessage("error", "Web3/MetaMask functionality is temporarily disabled");
}

export function getWeb3OnboardWalletsOptions() {
  return [];
}

export function initWeb3Onboard(application, provider) {
  showMessage("error", "Web3-Onboard functionality is temporarily disabled");
  return null;
}

export async function authViaWeb3Onboard(application, provider, method) {
  showMessage("error", "Web3-Onboard functionality is temporarily disabled");
}
