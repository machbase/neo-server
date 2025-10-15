# Preparing Windows Environment for Installation

## Open Firewall Port

If you install Machbase in Windows, you must open the port that Machbase uses in the Windows Firewall.

In general, Machbase uses  two ports: **5656 and 5001**

1. To register the port on the  firewall , select Control Panel - Windows Firewall or Windows Defender Firewall. 
    On the Run screen, click the "Advanced Settings" menu.

2. Under Advanced Settings, select **Inbound Rules - New Rule** and click.

3. When the New Rule Setup Wizard window is displayed, select the Port option as shown below and click Next.

4. Select the **TCP(T)** option , enter **5656,5001** in the **Specific Local Ports** field, and then click Next.

5. Select the **Allow The Connection** option and click **Next**.

6. Check **Domain** , **Private** , and **Public** and click **Next**.

7. Complete the **Name** and **Description** fields, and then click **Finish**.

    
