import { Component, Input, OnDestroy, OnInit } from '@angular/core';
import { PreviewTransaction, Transaction } from '../../../../../app.datatypes';
import { PriceService } from '../../../../../services/price.service';
import { SubscriptionLike } from 'rxjs';
import { BigNumber } from 'bignumber.js';
import { MatDialogConfig, MatDialog } from '@angular/material/dialog';
import { ChangeNoteComponent } from './change-note/change-note.component';

@Component({
  selector: 'app-transaction-info',
  templateUrl: './transaction-info.component.html',
  styleUrls: ['./transaction-info.component.scss'],
})
export class TransactionInfoComponent implements OnInit, OnDestroy {
  @Input() transaction: Transaction;
  @Input() isPreview: boolean;
  price: number;
  showInputsOutputs = false;

  private subscription: SubscriptionLike;

  constructor(private priceService: PriceService, private dialog: MatDialog) {
    this.subscription = this.priceService.price.subscribe(price => this.price = price);
  }

  get hoursText(): string {
    if (!this.transaction) {
      return '';
    }

    if (!this.isPreview) {
      if ((this.transaction as any).coinsMovedInternally) {
        return 'tx.hours-moved';
      } else if (this.transaction.balance.isGreaterThan(0)) {
        return 'tx.hours-received';
      }
    }

    return 'tx.hours-sent';
  }

  ngOnInit() {
    if (this.isPreview) {
      this.transaction.hoursSent = new BigNumber('0');
      this.transaction.outputs
        .filter(o => (<PreviewTransaction> this.transaction).to.find(addr => addr === o.address))
        .map(o => this.transaction.hoursSent = this.transaction.hoursSent.plus(new BigNumber(o.hours)));
    }
  }

  ngOnDestroy() {
    this.subscription.unsubscribe();
  }

  toggleInputsOutputs(event) {
    event.preventDefault();

    this.showInputsOutputs = !this.showInputsOutputs;
  }

  editNote() {
    ChangeNoteComponent.openDialog(this.dialog, this.transaction).afterClosed().subscribe(newNote => {
      if (newNote || newNote === '') {
        this.transaction.note = newNote;
      }
    });
  }
}
