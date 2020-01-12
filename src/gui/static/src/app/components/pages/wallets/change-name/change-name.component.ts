import { mergeMap } from 'rxjs/operators';
import { Component, OnInit, Inject, ViewChild, OnDestroy } from '@angular/core';
import { WalletService } from '../../../../services/wallet.service';
import { FormBuilder, Validators, FormGroup } from '@angular/forms';
import { MAT_DIALOG_DATA, MatDialogRef, MatDialog, MatDialogConfig } from '@angular/material/dialog';
import { Wallet, NormalTransaction } from '../../../../app.datatypes';
import { ButtonComponent } from '../../../layout/button/button.component';
import { MessageIcons } from '../../../layout/hardware-wallet/hw-message/hw-message.component';
import { HwWalletService } from '../../../../services/hw-wallet.service';
import { TranslateService } from '@ngx-translate/core';
import { getHardwareWalletErrorMsg } from '../../../../utils/errors';
import { SubscriptionLike } from 'rxjs';
import { MsgBarService } from '../../../../services/msg-bar.service';
import { AppConfig } from '../../../../app.config';

enum States {
  Initial,
  WaitingForConfirmation,
}

export class ChangeNameData {
  wallet: Wallet;
  newName: string;
}

export class ChangeNameErrorResponse {
  errorMsg: string;
}

@Component({
  selector: 'app-change-name',
  templateUrl: './change-name.component.html',
  styleUrls: ['./change-name.component.scss'],
})
export class ChangeNameComponent implements OnInit, OnDestroy {
  @ViewChild('button', { static: false }) button: ButtonComponent;
  form: FormGroup;
  currentState: States = States.Initial;
  states = States;
  msgIcons = MessageIcons;
  maxHwWalletLabelLength = HwWalletService.maxLabelLength;
  showCharactersWarning = false;
  working = false;

  private newLabel: string;
  private hwConnectionSubscription: SubscriptionLike;
  private operationSubscription: SubscriptionLike;

  public static openDialog(dialog: MatDialog, data: ChangeNameData, smallSize: boolean): MatDialogRef<ChangeNameComponent, any> {
    const config = new MatDialogConfig();
    config.data = data;
    config.autoFocus = true;
    config.width = smallSize ? '400px' : AppConfig.mediumModalWidth;

    return dialog.open(ChangeNameComponent, config);
  }

  constructor(
    public dialogRef: MatDialogRef<ChangeNameComponent>,
    @Inject(MAT_DIALOG_DATA) private data: ChangeNameData,
    private formBuilder: FormBuilder,
    private walletService: WalletService,
    private hwWalletService: HwWalletService,
    private translateService: TranslateService,
    private msgBarService: MsgBarService,
  ) {}

  ngOnInit() {
    if (!this.data.newName) {
      this.form = this.formBuilder.group({
        label: [this.data.wallet.label, Validators.required],
      });
    } else {
      this.finishRenaming(this.data.newName);
    }

    if (this.data.wallet.isHardware) {
      this.showCharactersWarning = true;

      this.hwConnectionSubscription = this.hwWalletService.walletConnectedAsyncEvent.subscribe(connected => {
        if (!connected) {
          this.closePopup();
        }
      });
    }
  }

  ngOnDestroy() {
    this.msgBarService.hide();
    if (this.hwConnectionSubscription) {
      this.hwConnectionSubscription.unsubscribe();
    }
    if (this.operationSubscription) {
      this.operationSubscription.unsubscribe();
    }
  }

  closePopup() {
    this.dialogRef.close();
  }

  rename() {
    if (!this.form.valid || this.button.isLoading()) {
      return;
    }

    this.msgBarService.hide();
    this.button.setLoading();

    this.finishRenaming(this.form.value.label);
  }

  private finishRenaming(newLabel) {
    this.working = true;
    this.newLabel = newLabel;

    if (!this.data.wallet.isHardware) {
      this.operationSubscription = this.walletService.renameWallet(this.data.wallet, this.newLabel)
        .subscribe(() => {
          this.working = false;
          this.dialogRef.close(this.newLabel);
          setTimeout(() => this.msgBarService.showDone('common.changes-made'));
        }, e => {
          this.working = false;
          this.msgBarService.showError(e);
          if (this.button) {
            this.button.resetState();
          }
        });
    } else {
      if (this.data.newName) {
        this.currentState = States.WaitingForConfirmation;
      }

      this.operationSubscription = this.hwWalletService.checkIfCorrectHwConnected(this.data.wallet.addresses[0].address).pipe(
        mergeMap(() => {
          this.currentState = States.WaitingForConfirmation;

          return this.hwWalletService.changeLabel(this.newLabel);
        }))
        .subscribe(
          () => {
            this.working = false;
            this.data.wallet.label = this.newLabel;
            this.walletService.saveHardwareWallets();
            this.dialogRef.close(this.newLabel);

            if (!this.data.newName) {
              setTimeout(() => this.msgBarService.showDone('common.changes-made'));
            }
          },
          err => {
            this.working = false;
            if (this.data.newName) {
              const response = new ChangeNameErrorResponse();
              response.errorMsg = getHardwareWalletErrorMsg(this.translateService, err);
              this.dialogRef.close(response);
            } else {
              this.msgBarService.showError(getHardwareWalletErrorMsg(this.translateService, err));
              this.currentState = States.Initial;
              if (this.button) {
                this.button.resetState();
              }
            }
          },
        );
    }
  }
}
