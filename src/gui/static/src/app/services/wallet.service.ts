import { throwError as observableThrowError, of, Subject, Observable, ReplaySubject, Subscription, BehaviorSubject } from 'rxjs';
import { concat, delay, retryWhen, first, take, mergeMap, catchError, map } from 'rxjs/operators';
import { Injectable } from '@angular/core';
import { ApiService } from './api.service';
import { Address, NormalTransaction, PreviewTransaction, Wallet } from '../app.datatypes';
import { BigNumber } from 'bignumber.js';
import { HwWalletService, HwOutput, HwInput } from './hw-wallet.service';
import { TranslateService } from '@ngx-translate/core';
import { StorageService, StorageType } from './storage.service';
import { TxEncoder } from '../utils/tx-encoder';
import { BalanceAndOutputsService } from './wallet-operations/balance-and-outputs.service';

export interface PendingTransactions {
  user: any[];
  all: any[];
}

@Injectable()
export class WalletService {

  addresses: Address[];
  wallets: Subject<Wallet[]> = new ReplaySubject<Wallet[]>(1);
  pendingTxs: Subject<PendingTransactions> = new ReplaySubject<PendingTransactions>(1);
  dataRefreshSubscription: Subscription;

  initialLoadFailed: BehaviorSubject<boolean> = new BehaviorSubject<boolean>(false);

  constructor(
    private apiService: ApiService,
    private hwWalletService: HwWalletService,
    private translate: TranslateService,
    private storageService: StorageService,
    private balanceAndOutputsService: BalanceAndOutputsService,
  ) {
    this.loadData();
    this.balanceAndOutputsService.refreshBalance();
  }

  addressesAsString(): Observable<string> {
    return this.allAddresses().pipe(map(addrs => addrs.map(addr => addr.address)), map(addrs => addrs.join(',')));
  }

  all(): Observable<Wallet[]> {
    return this.wallets.asObservable();
  }

  allAddresses(): Observable<any[]> {
    return this.all().pipe(map(wallets => wallets.reduce((array, wallet) => array.concat(wallet.addresses), [])));
  }

  pendingTransactions(): Observable<PendingTransactions> {
    return this.pendingTxs.asObservable();
  }

  createTransaction(
    wallet: Wallet|null,
    addresses: string[]|null,
    unspents: string[]|null,
    destinations: any[],
    hoursSelection: any,
    changeAddress: string|null,
    password: string|null,
    unsigned: boolean): Observable<PreviewTransaction> {

    if (unspents) {
      addresses = null;
    }

    if (wallet && wallet.isHardware && !changeAddress) {
      changeAddress = wallet.addresses[0].address;
    }

    const useV2Endpoint = !wallet || !!wallet.isHardware;

    const params = {
      hours_selection: hoursSelection,
      wallet_id: wallet ? wallet.filename : null,
      password: password,
      addresses: addresses,
      unspents: unspents,
      to: destinations,
      change_address: changeAddress,
    };

    if (!useV2Endpoint) {
      params['unsigned'] = unsigned;
    }

    let response: Observable<PreviewTransaction> = this.apiService.post(
      useV2Endpoint ? 'transaction' : 'wallet/transaction',
      params,
      {
        json: true,
      },
      useV2Endpoint,
    ).pipe(map(transaction => {
      const data = useV2Endpoint ? transaction.data : transaction;

      if (wallet && wallet.isHardware) {
        if (data.transaction.inputs.length > 8) {
          throw new Error(this.translate.instant('hardware-wallet.errors.too-many-inputs-outputs'));
        }
        if (data.transaction.outputs.length > 8) {
          throw new Error(this.translate.instant('hardware-wallet.errors.too-many-inputs-outputs'));
        }
      }

      return {
        ...data.transaction,
        hoursBurned: new BigNumber(data.transaction.fee),
        encoded: data.encoded_transaction,
        innerHash: data.transaction.inner_hash,
      };
    }));

    if (wallet && wallet.isHardware && !unsigned) {
      let unsignedTx: PreviewTransaction;

      response = response.pipe(mergeMap(transaction => {
        unsignedTx = transaction;

        return this.signTransaction(wallet, null, transaction);
      })).pipe(map(signedTx => {
        unsignedTx.encoded = signedTx.encoded;

        return unsignedTx;
      }));
    }

    return response;
  }

  signTransaction(
    wallet: Wallet,
    password: string|null,
    transaction: PreviewTransaction,
    rawTransactionString = ''): Observable<PreviewTransaction> {

    if (!wallet.isHardware) {
      return this.apiService.post(
        'wallet/transaction/sign',
        {
          wallet_id: wallet ? wallet.filename : null,
          password: password,
          encoded_transaction: rawTransactionString ? rawTransactionString : transaction.encoded,
        },
        {
          json: true,
        },
        true,
      ).pipe(map(response => {
        return {
          ...response.data.transaction,
          hoursBurned: new BigNumber(response.data.transaction.fee),
          encoded: response.data.encoded_transaction,
        };
      }));

    } else {
      if (rawTransactionString) {
        throw new Error('Raw transactions not allowed.');
      }

      const txOutputs = [];
      const txInputs = [];
      const hwOutputs: HwOutput[] = [];
      const hwInputs: HwInput[] = [];

      transaction.outputs.forEach(output => {
        txOutputs.push({
          address: output.address,
          coins: parseInt(new BigNumber(output.coins).multipliedBy(1000000).toFixed(0), 10),
          hours: parseInt(output.hours, 10),
        });

        hwOutputs.push({
          address: output.address,
          coins: new BigNumber(output.coins).toString(),
          hours: new BigNumber(output.hours).toFixed(0),
        });
      });

      if (hwOutputs.length > 1) {
        for (let i = txOutputs.length - 1; i >= 0; i--) {
          if (hwOutputs[i].address === wallet.addresses[0].address) {
            hwOutputs[i].address_index = 0;
            break;
          }
        }
      }

      const addressesMap: Map<string, number> = new Map<string, number>();
      wallet.addresses.forEach((address, i) => addressesMap.set(address.address, i));

      transaction.inputs.forEach(input => {
        txInputs.push({
          hash: input.uxid,
          secret: '',
          address: input.address,
          address_index: addressesMap.get(input.address),
          calculated_hours: parseInt(input.calculated_hours, 10),
          coins: parseInt(input.coins, 10),
        });

        hwInputs.push({
          hash: input.uxid,
          index: addressesMap.get(input.address),
        });
      });

      return this.hwWalletService.signTransaction(hwInputs, hwOutputs).pipe(mergeMap(signatures => {
        const rawTransaction = TxEncoder.encode(
          hwInputs,
          hwOutputs,
          signatures.rawResponse,
          transaction.innerHash,
        );

        return of({
          ...transaction,
          encoded: rawTransaction,
        });
      }));
    }
  }

  injectTransaction(encodedTx: string, note: string): Observable<boolean> {
    return this.apiService.post('injectTransaction', { rawtx: encodedTx }, { json: true }).pipe(
      mergeMap(txId => {
        setTimeout(() => this.balanceAndOutputsService.refreshBalance(), 32);

        if (!note) {
          return of(false);
        } else {
          return this.storageService.store(StorageType.NOTES, txId, note).pipe(
            retryWhen(errors => errors.pipe(delay(1000), take(3), concat(observableThrowError(-1)))),
            catchError(err => err === -1 ? of(-1) : err),
            map(result => result === -1 ? false : true));
        }
      }));
  }

  transactions(): Observable<NormalTransaction[]> {
    let wallets: Wallet[];
    let transactions: NormalTransaction[];
    const addressesMap: Map<string, boolean> = new Map<string, boolean>();


    return this.wallets.pipe(first(), mergeMap(w => {
      wallets = w;

      return this.allAddresses().pipe(first());
    }), mergeMap(addresses => {
      this.addresses = addresses;
      addresses.map(add => addressesMap.set(add.address, true));

      return this.apiService.getTransactions(addresses);
    }), mergeMap(recoveredTransactions => {
      transactions = recoveredTransactions;

      return this.storageService.get(StorageType.NOTES, null);
    }), map(notes => {
      const notesMap: Map<string, string> = new Map<string, string>();
      Object.keys(notes.data).forEach(key => {
        notesMap.set(key, notes.data[key]);
      });

      return transactions
        .sort((a, b) =>  b.timestamp - a.timestamp)
        .map(transaction => {
          const outgoing = transaction.inputs.some(input => addressesMap.has(input.owner));

          const relevantAddresses: Map<string, boolean> = new Map<string, boolean>();
          transaction.balance = new BigNumber('0');
          transaction.hoursSent = new BigNumber('0');

          if (!outgoing) {
            transaction.outputs.map(output => {
              if (addressesMap.has(output.dst)) {
                relevantAddresses.set(output.dst, true);
                transaction.balance = transaction.balance.plus(output.coins);
                transaction.hoursSent = transaction.hoursSent.plus(output.hours);
              }
            });
          } else {
            const possibleReturnAddressesMap: Map<string, boolean> = new Map<string, boolean>();
            transaction.inputs.map(input => {
              if (addressesMap.has(input.owner)) {
                relevantAddresses.set(input.owner, true);
                wallets.map(wallet => {
                  if (wallet.addresses.some(add => add.address === input.owner)) {
                    wallet.addresses.map(add => possibleReturnAddressesMap.set(add.address, true));
                  }
                });
              }
            });

            transaction.outputs.map(output => {
              if (!possibleReturnAddressesMap.has(output.dst)) {
                transaction.balance = transaction.balance.minus(output.coins);
                transaction.hoursSent = transaction.hoursSent.plus(output.hours);
              }
            });

            if (transaction.balance.isEqualTo(0)) {
              transaction.coinsMovedInternally = true;
              const inputAddressesMap: Map<string, boolean> = new Map<string, boolean>();

              transaction.inputs.map(input => {
                inputAddressesMap.set(input.owner, true);
              });

              transaction.outputs.map(output => {
                if (!inputAddressesMap.has(output.dst)) {
                  relevantAddresses.set(output.dst, true);
                  transaction.balance = transaction.balance.plus(output.coins);
                  transaction.hoursSent = transaction.hoursSent.plus(output.hours);
                }
              });
            }
          }

          relevantAddresses.forEach((value, key) => {
            transaction.addresses.push(key);
          });

          let inputsHours = new BigNumber('0');
          transaction.inputs.map(input => inputsHours = inputsHours.plus(new BigNumber(input.calculated_hours)));
          let outputsHours = new BigNumber('0');
          transaction.outputs.map(output => outputsHours = outputsHours.plus(new BigNumber(output.hours)));
          transaction.hoursBurned = inputsHours.minus(outputsHours);

          const txNote = notesMap.get(transaction.txid);
          if (txNote) {
            transaction.note = txNote;
          }

          return transaction;
        });
    }));
  }

  saveHardwareWallets() {
    this.wallets.pipe(first()).subscribe(wallets => {
      const hardwareWallets: Wallet[] = [];

      wallets.map(wallet => {
        if (wallet.isHardware) {
          hardwareWallets.push(this.createHardwareWalletData(
            wallet.label,
            wallet.addresses.map(address => {
              return { address: address.address, confirmed: address.confirmed };
            }),
            wallet.hasHwSecurityWarnings,
            wallet.stopShowingHwSecurityPopup,
          ));
        }
      });

      this.hwWalletService.saveWalletsData(JSON.stringify(hardwareWallets));

      this.wallets.next(wallets);
    });
  }

  private createHardwareWalletData(label: string, addresses: {address: string, confirmed: boolean}[], hasHwSecurityWarnings: boolean, stopShowingHwSecurityPopup: boolean): Wallet {
    return {
      label: label,
      filename: '',
      hasHwSecurityWarnings: hasHwSecurityWarnings,
      stopShowingHwSecurityPopup: stopShowingHwSecurityPopup,
      coins: null,
      hours: null,
      addresses: addresses.map(address => {
        return {
          address: address.address,
          coins: null,
          hours: null,
          confirmed: address.confirmed,
        };
      }),
      encrypted: false,
      isHardware: true,
    };
  }

  private loadData(): void {
    let wallets: Wallet[] = [];
    let softwareWallets: Wallet[] = [];

    this.apiService.getWallets().pipe(first(), mergeMap(recoveredWallets => {
      softwareWallets = recoveredWallets;

      if (this.hwWalletService.hwWalletCompatibilityActivated) {
        return this.loadHardwareWallets(wallets);
      }

      return of(null);

    })).subscribe(() => {
      wallets = wallets.concat(softwareWallets);
      this.wallets.next(wallets);
    }, () => this.initialLoadFailed.next(true));
  }

  private loadHardwareWallets(wallets: Wallet[]): Observable<any> {
    return this.hwWalletService.getSavedWalletsData().pipe(map(storedWallets => {
      if (storedWallets) {
        const loadedWallets: Wallet[] = JSON.parse(storedWallets);
        loadedWallets.map(wallet => wallets.push(wallet));
      }

      return null;
    }));
  }
}
