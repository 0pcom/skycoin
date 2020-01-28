import { Component, OnDestroy } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { SubscriptionLike } from 'rxjs';
import { BalanceAndOutputsService } from 'src/app/services/wallet-operations/balance-and-outputs.service';

@Component({
  selector: 'app-outputs',
  templateUrl: './outputs.component.html',
  styleUrls: ['./outputs.component.scss'],
})
export class OutputsComponent implements OnDestroy {
  wallets: any[]|null;

  private outputsSubscription: SubscriptionLike;
  private lastRouteParams: any;

  constructor(
    private route: ActivatedRoute,
    private balanceAndOutputsService: BalanceAndOutputsService,
  ) {
    route.queryParams.subscribe(params => {
      this.wallets = null;
      this.lastRouteParams = params;
      this.balanceAndOutputsService.refreshBalance();
    });
    this.loadData();
  }

  ngOnDestroy() {
    this.outputsSubscription.unsubscribe();
  }

  loadData() {
    const addr = this.lastRouteParams['addr'];

    this.outputsSubscription = this.balanceAndOutputsService.outputsWithWallets().subscribe(wallets => {
      this.wallets = wallets
        .map(wallet => Object.assign({}, wallet))
        .map(wallet => {
          wallet.addresses = wallet.addresses.filter(address => {
            if (address.outputs.length > 0) {
              return addr ? address.address === addr : true;
            }
          });

          return wallet;
        })
        .filter(wallet => wallet.addresses.length > 0);
    });
  }
}
