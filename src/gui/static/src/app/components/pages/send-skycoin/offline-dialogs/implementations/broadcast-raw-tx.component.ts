import { Component, OnInit, OnDestroy } from '@angular/core';
import { MatDialogRef } from '@angular/material/dialog';
import { OfflineDialogsBaseComponent, OfflineDialogsStates } from '../offline-dialogs-base.component';
import { MsgBarService } from '../../../../../services/msg-bar.service';
import { FormBuilder } from '@angular/forms';
import { SubscriptionLike } from 'rxjs';
import { WalletService } from '../../../../../services/wallet.service';
import { parseResponseMessage } from '../../../../../utils/errors';

@Component({
  selector: 'app-broadcast-raw-tx',
  templateUrl: '../offline-dialogs-base.component.html',
  styleUrls: ['../offline-dialogs-base.component.scss'],
})
export class BroadcastRawTxComponent extends OfflineDialogsBaseComponent implements OnInit, OnDestroy {
  title = 'offline-transactions.broadcast-tx.title';
  text = 'offline-transactions.broadcast-tx.text';
  inputLabel = 'offline-transactions.broadcast-tx.input-label';
  cancelButtonText = 'offline-transactions.broadcast-tx.cancel';
  okButtonText = 'offline-transactions.broadcast-tx.send';
  validateForm = true;

  private operationSubscription: SubscriptionLike;

  constructor(
    public dialogRef: MatDialogRef<BroadcastRawTxComponent>,
    private walletService: WalletService,
    private msgBarService: MsgBarService,
    formBuilder: FormBuilder,
  ) {
    super(formBuilder);

    this.currentState = OfflineDialogsStates.ShowingForm;
  }

  ngOnInit() {
    this.form.get('dropdown').setValue('dummy');
  }

  ngOnDestroy() {
    this.closeOperationSubscription();
  }

  cancelPressed() {
    this.dialogRef.close();
  }

  okPressed() {
    if (this.working) {
      return;
    }

    this.msgBarService.hide();
    this.working = true;
    this.okButton.setLoading();

    this.closeOperationSubscription();
    this.operationSubscription = this.walletService.injectTransaction(this.form.get('input').value, null).subscribe(response => {
      this.walletService.startDataRefreshSubscription();

      this.msgBarService.showDone('offline-transactions.broadcast-tx.sent');
      this.cancelPressed();
    }, error => {
      this.working = false;
      this.okButton.resetState();

      const parsedErrorMsg = parseResponseMessage(error);
      if (parsedErrorMsg !== error) {
        this.msgBarService.showError(parsedErrorMsg);
      } else if (error && error.error && error.error.error && error.error.error.message) {
        this.msgBarService.showError(error.error.error.message);
      } else {
        this.msgBarService.showError('offline-transactions.broadcast-tx.error');
      }
    });
  }

  closeOperationSubscription() {
    if (this.operationSubscription) {
      this.operationSubscription.unsubscribe();
    }
  }
}
