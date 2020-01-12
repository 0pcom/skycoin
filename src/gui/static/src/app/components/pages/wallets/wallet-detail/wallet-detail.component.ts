import { Component, Input, OnDestroy, OnInit } from '@angular/core';
import { Wallet, ConfirmationData } from '../../../../app.datatypes';
import { WalletService } from '../../../../services/wallet.service';
import { MatDialog, MatDialogConfig, MatDialogRef } from '@angular/material/dialog';
import { ChangeNameComponent, ChangeNameData } from '../change-name/change-name.component';
import { QrCodeComponent, QrDialogConfig } from '../../../layout/qr-code/qr-code.component';
import { PasswordDialogComponent, PasswordDialogParams } from '../../../layout/password-dialog/password-dialog.component';
import { getHardwareWalletErrorMsg } from '../../../../utils/errors';
import { NumberOfAddressesComponent } from '../number-of-addresses/number-of-addresses';
import { TranslateService } from '@ngx-translate/core';
import { HwWalletService } from '../../../../services/hw-wallet.service';
import { Observable, SubscriptionLike } from 'rxjs';
import { showConfirmationModal, copyTextToClipboard } from '../../../../utils';
import { AppConfig } from '../../../../app.config';
import { Router } from '@angular/router';
import { HwConfirmAddressDialogComponent, AddressConfirmationParams } from '../../../layout/hardware-wallet/hw-confirm-address-dialog/hw-confirm-address-dialog.component';
import { MsgBarService } from '../../../../services/msg-bar.service';
import { ApiService } from '../../../../services/api.service';
import { mergeMap, first } from 'rxjs/operators';
import { AddressOptionsComponent, AddressOptions } from './address-options/address-options.component';

@Component({
  selector: 'app-wallet-detail',
  templateUrl: './wallet-detail.component.html',
  styleUrls: ['./wallet-detail.component.scss'],
})
export class WalletDetailComponent implements OnDestroy {
  @Input() wallet: Wallet;

  confirmingIndex = null;
  workingWithAddresses = false;
  preparingToEdit = false;

  private howManyAddresses: number;
  private editSubscription: SubscriptionLike;
  private confirmSubscription: SubscriptionLike;
  private txHistorySubscription: SubscriptionLike;

  constructor(
    private dialog: MatDialog,
    private walletService: WalletService,
    private msgBarService: MsgBarService,
    private hwWalletService: HwWalletService,
    private translateService: TranslateService,
    private router: Router,
    private apiService: ApiService,
  ) { }

  ngOnDestroy() {
    this.msgBarService.hide();
    if (this.editSubscription) {
      this.editSubscription.unsubscribe();
    }
    if (this.confirmSubscription) {
      this.confirmSubscription.unsubscribe();
    }
    if (this.txHistorySubscription) {
      this.txHistorySubscription.unsubscribe();
    }
  }

  editWallet() {
    this.msgBarService.hide();

    if (this.wallet.isHardware) {
      if (this.preparingToEdit) {
        return;
      }

      this.preparingToEdit = true;
      this.editSubscription = this.hwWalletService.checkIfCorrectHwConnected(this.wallet.addresses[0].address)
        .pipe(mergeMap(() => this.walletService.getHwFeaturesAndUpdateData(this.wallet)))
        .subscribe(
          response => {
            this.continueEditWallet();
            this.preparingToEdit = false;

            if (response.walletNameUpdated) {
              this.msgBarService.showWarning('hardware-wallet.general.name-updated');
            }
          },
          err => {
            this.msgBarService.showError(getHardwareWalletErrorMsg(this.translateService, err));
            this.preparingToEdit = false;
          },
        );
    } else {
      this.continueEditWallet();
    }
  }

  openAddressOptions() {
    if (this.workingWithAddresses) {
      return;
    }

    AddressOptionsComponent.openDialog(this.dialog).afterClosed().subscribe(result => {
      if (result === AddressOptions.new) {
        this.newAddress();
      } else if (result === AddressOptions.scan) {
        this.scanAddresses();
      }
    });
  }

  newAddress() {
    if (this.workingWithAddresses) {
      return;
    }

    if (this.wallet.isHardware && this.wallet.addresses.length >= AppConfig.maxHardwareWalletAddresses) {
      const confirmationData: ConfirmationData = {
        text: 'wallet.max-hardware-wallets-error',
        headerText: 'errors.error',
        confirmButtonText: 'confirmation.close',
      };
      showConfirmationModal(this.dialog, confirmationData);

      return;
    }

    this.msgBarService.hide();

    if (!this.wallet.isHardware) {
      const maxAddressesGap = 20;

      const eventFunction = (howManyAddresses, callback) => {
        this.howManyAddresses = howManyAddresses;

        let lastWithBalance = 0;
        this.wallet.addresses.forEach((address, i) => {
          if (address.coins.isGreaterThan(0)) {
            lastWithBalance = i;
          }
        });

        if ((this.wallet.addresses.length - (lastWithBalance + 1)) + howManyAddresses < maxAddressesGap) {
          callback(true);
          this.continueNewAddress();
        } else {
          this.txHistorySubscription = this.apiService.getTransactions(this.wallet.addresses).pipe(first()).subscribe(transactions => {
            const AddressesWithTxs = new Map<string, boolean>();

            transactions.forEach(transaction => {
              transaction.outputs.forEach(output => {
                if (!AddressesWithTxs.has(output.dst)) {
                  AddressesWithTxs.set(output.dst, true);
                }
              });
            });

            let lastWithTxs = 0;
            this.wallet.addresses.forEach((address, i) => {
              if (AddressesWithTxs.has(address.address)) {
                lastWithTxs = i;
              }
            });

            if ((this.wallet.addresses.length - (lastWithTxs + 1)) + howManyAddresses < maxAddressesGap) {
              callback(true);
              this.continueNewAddress();
            } else {
              const confirmationData: ConfirmationData = {
                text: 'wallet.add-many-confirmation',
                headerText: 'confirmation.header-text',
                confirmButtonText: 'confirmation.confirm-button',
                cancelButtonText: 'confirmation.cancel-button',
              };

              showConfirmationModal(this.dialog, confirmationData).afterClosed().subscribe(confirmationResult => {
                if (confirmationResult) {
                  callback(true);
                  this.continueNewAddress();
                } else {
                  callback(false);
                }
              });
            }
          }, () => callback(false, true));
        }
      };

      NumberOfAddressesComponent.openDialog(this.dialog, eventFunction);
    } else {
      this.howManyAddresses = 1;
      this.continueNewAddress();
    }
  }

  toggleEmpty() {
    this.wallet.hideEmpty = !this.wallet.hideEmpty;
  }

  deleteWallet() {
    this.msgBarService.hide();

    const confirmationData: ConfirmationData = {
      text: this.translateService.instant('wallet.delete-confirmation', {name: this.wallet.label}),
      headerText: 'confirmation.header-text',
      checkboxText: 'wallet.delete-confirmation-check',
      confirmButtonText: 'confirmation.confirm-button',
      cancelButtonText: 'confirmation.cancel-button',
    };

    showConfirmationModal(this.dialog, confirmationData).afterClosed().subscribe(confirmationResult => {
      if (confirmationResult) {
        this.walletService.deleteHardwareWallet(this.wallet).subscribe(result => {
          if (result) {
            this.walletService.all().pipe(first()).subscribe(wallets => {
              if (wallets.length === 0) {
                setTimeout(() => this.router.navigate(['/wizard']), 500);
              }
            });
          }
        });
      }
    });
  }

  toggleEncryption() {
    const params: PasswordDialogParams = {
      confirm: !this.wallet.encrypted,
      title: this.wallet.encrypted ? 'wallet.decrypt' : 'wallet.encrypt',
      description: this.wallet.encrypted ? 'wallet.decrypt-warning' : 'wallet.new.encrypt-warning',
      warning: this.wallet.encrypted,
      wallet: this.wallet.encrypted ? this.wallet : null,
    };

    PasswordDialogComponent.openDialog(this.dialog, params, false).componentInstance.passwordSubmit
      .subscribe(passwordDialog => {
        this.walletService.toggleEncryption(this.wallet, passwordDialog.password).subscribe(() => {
          passwordDialog.close();
          setTimeout(() => this.msgBarService.showDone('common.changes-made'));
        }, e => passwordDialog.error(e));
      });
  }

  confirmAddress(address, addressIndex, showCompleteConfirmation) {
    if (this.confirmingIndex !== null) {
      return;
    }

    this.confirmingIndex = addressIndex;
    this.msgBarService.hide();

    if (this.confirmSubscription) {
      this.confirmSubscription.unsubscribe();
    }

    this.confirmSubscription = this.hwWalletService.checkIfCorrectHwConnected(this.wallet.addresses[0].address).subscribe(response => {
      const data = new AddressConfirmationParams();
      data.address = address;
      data.addressIndex = addressIndex;
      data.showCompleteConfirmation = showCompleteConfirmation;

      const config = new MatDialogConfig();
      config.width = '566px';
      config.autoFocus = false;
      config.data = data;
      this.dialog.open(HwConfirmAddressDialogComponent, config);

      this.confirmingIndex = null;
    }, err => {
      this.msgBarService.showError(getHardwareWalletErrorMsg(this.translateService, err));
      this.confirmingIndex = null;
    });
  }

  copyAddress(event, address, duration = 500) {
    event.stopPropagation();

    if (address.copying) {
      return;
    }

    copyTextToClipboard(address.address);
    address.copying = true;

    setTimeout(function() {
      address.copying = false;
    }, duration);
  }

  showQrCode(event, address: string) {
    event.stopPropagation();

    const config: QrDialogConfig = { address };
    QrCodeComponent.openDialog(this.dialog, config);
  }

  private scanAddresses() {
    if (this.workingWithAddresses) {
      return;
    }

    this.workingWithAddresses = true;

    if (!this.wallet.isHardware && this.wallet.encrypted) {
      const dialogRef = PasswordDialogComponent.openDialog(this.dialog, { wallet: this.wallet });
      dialogRef.afterClosed().subscribe(() => this.workingWithAddresses = false);
      dialogRef.componentInstance.passwordSubmit.subscribe(passwordDialog => {
        this.walletService.scanAddresses(this.wallet, passwordDialog.password).subscribe(result => {
          passwordDialog.close();

          setTimeout(() => {
            if (result) {
              this.msgBarService.showDone('wallet.scan-addresses.done-with-new-addresses');
            } else {
              this.msgBarService.showWarning('wallet.scan-addresses.done-without-new-addresses');
            }
          });
        }, error => {
          passwordDialog.error(error);
        });
      });
    } else {
      this.walletService.scanAddresses(this.wallet).subscribe(result => {
        if (result) {
          this.msgBarService.showDone('wallet.scan-addresses.done-with-new-addresses');
        } else {
          this.msgBarService.showWarning('wallet.scan-addresses.done-without-new-addresses');
        }
        this.workingWithAddresses = false;
      }, err => {
        if (!this.wallet.isHardware ) {
          this.msgBarService.showError(err);
        } else {
          this.msgBarService.showError(getHardwareWalletErrorMsg(this.translateService, err));
        }
        this.workingWithAddresses = false;
      });
    }
  }

  private continueNewAddress() {
    this.workingWithAddresses = true;

    if (!this.wallet.isHardware && this.wallet.encrypted) {
      const dialogRef = PasswordDialogComponent.openDialog(this.dialog, { wallet: this.wallet });
      dialogRef.afterClosed().subscribe(() => this.workingWithAddresses = false);
      dialogRef.componentInstance.passwordSubmit
        .subscribe(passwordDialog => {
          this.walletService.addAddress(this.wallet, this.howManyAddresses, passwordDialog.password)
            .subscribe(() => passwordDialog.close(), error => passwordDialog.error(error));
        });
    } else {

      let procedure: Observable<any>;

      if (this.wallet.isHardware ) {
        procedure = this.hwWalletService.checkIfCorrectHwConnected(this.wallet.addresses[0].address).pipe(mergeMap(
          () => this.walletService.addAddress(this.wallet, this.howManyAddresses),
        ));
      } else {
        procedure = this.walletService.addAddress(this.wallet, this.howManyAddresses);
      }

      procedure.subscribe(() => this.workingWithAddresses = false,
        err => {
          if (!this.wallet.isHardware ) {
            this.msgBarService.showError(err);
          } else {
            this.msgBarService.showError(getHardwareWalletErrorMsg(this.translateService, err));
          }
          this.workingWithAddresses = false;
        },
      );
    }
  }

  private continueEditWallet() {
    const data = new ChangeNameData();
    data.wallet = this.wallet;
    ChangeNameComponent.openDialog(this.dialog, data, false);
  }
}
